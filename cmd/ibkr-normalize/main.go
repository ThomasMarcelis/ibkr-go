package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
	"google.golang.org/protobuf/encoding/protowire"
)

func main() {
	dir := flag.String("dir", "", "capture directory containing events.jsonl")
	rawOut := flag.String("out", "", "raw output file path (default: <dir>/raw.txt)")
	replayDir := flag.String("replay-dir", "", "directory for normalized replay artifacts (default: <dir>/replay)")
	transcriptOut := flag.String("transcript-out", "", "optional raw transcript skeleton output path")
	verify := flag.Bool("verify", false, "verify capture integrity and print a protocol-aware summary")
	flag.Parse()

	if err := runNormalize(os.Stdout, *dir, *rawOut, *replayDir, *transcriptOut, *verify); err != nil {
		log.Fatal(err)
	}
}

func runNormalize(out io.Writer, dir, rawOut, replayDir, transcriptOut string, verify bool) error {
	if dir == "" {
		return fmt.Errorf("-dir is required")
	}

	events, err := capturelog.LoadEvents(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return fmt.Errorf("load events: %w", err)
	}
	meta, err := capturelog.LoadMeta(filepath.Join(dir, "meta.json"))
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		return fmt.Errorf("normalize events: %w", err)
	}
	if verify {
		if transcriptOut != "" || rawOut != "" || replayDir != "" {
			return fmt.Errorf("-verify cannot be combined with output flags")
		}
		return writeVerification(out, dir, meta, events, replayEvents)
	}
	if rawOut == "" {
		rawOut = filepath.Join(dir, "raw.txt")
	}
	if replayDir == "" {
		replayDir = filepath.Join(dir, "replay")
	}

	file, err := os.OpenFile(rawOut, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- the caller explicitly selects this CLI output path.
	if err != nil {
		return fmt.Errorf("create raw output: %w", err)
	}

	for _, event := range events {
		kind := event.Kind
		if kind == "" {
			kind = capturelog.EventChunk
		}
		line := fmt.Sprintf("%s leg=%d kind=%s", event.At.Format("2006-01-02T15:04:05.000000000Z07:00"), event.Leg, kind)
		if kind == capturelog.EventChunk {
			data, err := capturelog.DecodeData(event)
			if err != nil {
				_ = file.Close()
				return fmt.Errorf("decode event: %w", err)
			}
			line = fmt.Sprintf("%s direction=%s len=%d hex=%s quoted=%q",
				line,
				event.Direction,
				event.Length,
				hex.EncodeToString(data),
				string(data),
			)
		}
		if _, err := fmt.Fprintln(file, line); err != nil {
			_ = file.Close()
			return fmt.Errorf("write raw output: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close raw output: %w", err)
	}

	if err := capturelog.WriteReplay(replayDir, dir, meta, replayEvents); err != nil {
		return fmt.Errorf("write replay: %w", err)
	}
	if transcriptOut != "" {
		if err := writeTranscriptSkeleton(transcriptOut, dir, meta, replayEvents); err != nil {
			return fmt.Errorf("write transcript skeleton: %w", err)
		}
	}
	return nil
}

func writeTranscriptSkeleton(path, captureDir string, meta capturelog.Meta, replayEvents []capturelog.ReplayEvent) (err error) {
	eventsData, err := os.ReadFile(filepath.Join(captureDir, "events.jsonl")) // #nosec G304 -- captureDir is the operator-selected input.
	if err != nil {
		return fmt.Errorf("read events provenance: %w", err)
	}
	identities, err := loadTranscriptRedactionIdentities(
		filepath.Join(captureDir, "driver_events.jsonl"),
		filepath.Join(captureDir, "transcript_redactions.jsonl"),
	)
	if err != nil {
		return fmt.Errorf("load transcript redaction identities: %w", err)
	}
	serverIdentities, err := transcriptServerIdentities(replayEvents)
	if err != nil {
		return fmt.Errorf("derive server redaction identities: %w", err)
	}
	redactions, err := transcriptRedactionsForIdentities(append(identities, serverIdentities...))
	if err != nil {
		return fmt.Errorf("build transcript redactions: %w", err)
	}
	serverVersions, err := transcriptServerVersions(replayEvents)
	if err != nil {
		return err
	}
	versionProvenance, err := formatTranscriptServerVersions(serverVersions)
	if err != nil {
		return err
	}

	// #nosec G304 -- path is the operator-selected transcript output.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create transcript skeleton: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close transcript skeleton: %w", closeErr))
		}
	}()

	if _, err := fmt.Fprintf(file, "# Exact capture %s for %s at %s.\n", filepath.Base(captureDir), meta.Scenario, versionProvenance); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "# events.jsonl sha256: %x.\n", sha256.Sum256(eventsData)); err != nil {
		return err
	}
	if redactions.changed {
		if _, err := fmt.Fprintln(file, "# Account, order-reference, execution-ID, OCA-group, permanent-ID, and submitter values are deterministically sanitized."); err != nil {
			return err
		}
	}
	frameState := newCaptureFrameState()
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if err := frameState.connect(event.Leg); err != nil {
				return err
			}
		case capturelog.EventDisconnect:
			if err := frameState.disconnect(event.Leg); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(file, "disconnect"); err != nil {
				return err
			}
		case capturelog.ReplayEventFrame:
			payload, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				return fmt.Errorf("decode replay frame: %w", err)
			}
			description, err := frameState.describe(event, payload)
			if err != nil {
				return err
			}
			if description.label == "server_info" {
				handshake, err := json.Marshal(struct {
					ServerVersion  int    `json:"server_version"`
					ConnectionTime string `json:"connection_time"`
				}{description.serverVersion, description.connectionTime})
				if err != nil {
					return fmt.Errorf("encode handshake: %w", err)
				}
				if _, err := fmt.Fprintf(file, "handshake %s\n", handshake); err != nil {
					return err
				}
				continue
			}
			if description.label == "version_range" ||
				event.Direction == "client" && description.msgID == protocol.OutStartAPI {
				continue
			}
			payload, err = redactions.applyFrame(event.Direction, description, payload)
			if err != nil {
				return fmt.Errorf("sanitize leg %d %s msg_id %s: %w", event.Leg, event.Direction, description.messageID(), err)
			}
			if _, err := fmt.Fprintf(file, "# leg=%d direction=%s msg_id=%s encoding=%s payload_len=%d\n", event.Leg, event.Direction, description.messageID(), description.encoding, len(payload)); err != nil {
				return err
			}
			frame, err := frameBytes(payload)
			if err != nil {
				return fmt.Errorf("frame replay payload: %w", err)
			}
			if _, err := fmt.Fprintf(file, "raw %s %s\n", event.Direction, base64.StdEncoding.EncodeToString(frame)); err != nil {
				return err
			}
		}
	}
	return nil
}

func transcriptServerVersions(replayEvents []capturelog.ReplayEvent) ([]int, error) {
	frameState := newCaptureFrameState()
	var serverVersions []int
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if err := frameState.connect(event.Leg); err != nil {
				return nil, err
			}
		case capturelog.EventDisconnect:
			if err := frameState.disconnect(event.Leg); err != nil {
				return nil, err
			}
		case capturelog.ReplayEventFrame:
			payload, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				return nil, fmt.Errorf("decode replay frame: %w", err)
			}
			description, err := frameState.describe(event, payload)
			if err != nil {
				return nil, err
			}
			if description.label != "server_info" {
				continue
			}
			if len(serverVersions) == 0 || serverVersions[len(serverVersions)-1] != description.serverVersion {
				serverVersions = append(serverVersions, description.serverVersion)
			}
		}
	}
	if len(serverVersions) == 0 {
		return nil, fmt.Errorf("capture has no server-info frame")
	}
	return serverVersions, nil
}

func formatTranscriptServerVersions(serverVersions []int) (string, error) {
	if len(serverVersions) == 0 {
		return "", errors.New("capture has no negotiated server version")
	}
	if len(serverVersions) == 1 {
		return fmt.Sprintf("server_version %d", serverVersions[0]), nil
	}
	for i := 1; i < len(serverVersions); i++ {
		if serverVersions[i] != serverVersions[i-1]+1 {
			return "", fmt.Errorf("capture negotiates nonconsecutive server versions %d then %d", serverVersions[i-1], serverVersions[i])
		}
	}
	return fmt.Sprintf("server_versions %d-%d", serverVersions[0], serverVersions[len(serverVersions)-1]), nil
}

type transcriptRedactions struct {
	replacements []transcriptReplacement
	changed      bool
}

type transcriptReplacement struct {
	kind transcriptIdentityKind
	wire transcriptIdentityWire
	from []byte
	to   []byte
}

type transcriptIdentityKind uint8

const (
	transcriptAccount transcriptIdentityKind = iota + 1
	transcriptOrderRef
	transcriptPermID
	transcriptOCAGroup
	transcriptExecID
	transcriptSubmitter
)

type transcriptIdentityWire uint8

const (
	transcriptString transcriptIdentityWire = iota + 1
	transcriptVarint
)

type transcriptDriverIdentity struct {
	Account   string `json:"account"`
	OrderID   int64  `json:"order_id"`
	OrderRef  string `json:"order_ref"`
	PermID    int64  `json:"perm_id"`
	OCAGroup  string `json:"oca_group"`
	ExecID    string `json:"exec_id"`
	Submitter string `json:"submitter"`
}

func loadTranscriptRedactions(paths ...string) (transcriptRedactions, error) {
	identities, err := loadTranscriptRedactionIdentities(paths...)
	if err != nil {
		return transcriptRedactions{}, err
	}
	return transcriptRedactionsForIdentities(identities)
}

func loadTranscriptRedactionIdentities(paths ...string) ([]transcriptDriverIdentity, error) {
	var identities []transcriptDriverIdentity
	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304,G703 -- paths are operator-selected local capture evidence.
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for line := range strings.Lines(string(data)) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var identity transcriptDriverIdentity
			if err := json.Unmarshal([]byte(line), &identity); err != nil {
				return nil, fmt.Errorf("decode redaction identity %s: %w", path, err)
			}
			identities = append(identities, identity)
		}
	}

	return identities, nil
}

func transcriptRedactionsForIdentities(identities []transcriptDriverIdentity) (transcriptRedactions, error) {
	redactions := transcriptRedactions{}
	seen := make(map[string]struct{})
	addString := func(kind transcriptIdentityKind, from, to string) error {
		if from == "" || from == to {
			return nil
		}
		if len(from) != len(to) {
			return fmt.Errorf("length-changing redaction %q -> %q", from, to)
		}
		key := fmt.Sprintf("s:%d:%s", kind, from)
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		redactions.replacements = append(redactions.replacements, transcriptReplacement{
			kind: kind, wire: transcriptString, from: []byte(from), to: []byte(to),
		})
		redactions.changed = true
		return nil
	}
	addToken := func(kind transcriptIdentityKind, value, prefix string, ordinal int) error {
		if value == "" {
			return nil
		}
		token, err := lengthPreservingTranscriptToken(len(value), prefix, ordinal)
		if err != nil {
			return err
		}
		return addString(kind, value, token)
	}

	orderRefOrdinal := 0
	ocaOrdinal := 0
	execOrdinal := 0
	submitterOrdinal := 0
	permOrderIDs := make(map[int64]int64)
	for _, identity := range identities {
		if identity.Account != "" {
			if len(identity.Account) != len("DU9000001") {
				return transcriptRedactions{}, fmt.Errorf("account %q is not length-compatible with DU9000001", identity.Account)
			}
			if err := addString(transcriptAccount, identity.Account, "DU9000001"); err != nil {
				return transcriptRedactions{}, err
			}
		}
		if identity.OrderRef != "" {
			orderRefOrdinal++
			if err := addToken(transcriptOrderRef, identity.OrderRef, "sanitized-order-ref-", orderRefOrdinal); err != nil {
				return transcriptRedactions{}, err
			}
		}
		if identity.ExecID != "" {
			execOrdinal++
			if err := addToken(transcriptExecID, identity.ExecID, "sanitized-exec-", execOrdinal); err != nil {
				return transcriptRedactions{}, err
			}
		}
		if identity.Submitter != "" {
			submitterOrdinal++
			if err := addToken(transcriptSubmitter, identity.Submitter, "paper-user-", submitterOrdinal); err != nil {
				return transcriptRedactions{}, err
			}
		}
		if identity.PermID < 0 {
			return transcriptRedactions{}, fmt.Errorf("negative perm id %d cannot be sanitized", identity.PermID)
		}
		if identity.PermID > 0 && identity.OrderID > 0 {
			if orderID, ok := permOrderIDs[identity.PermID]; ok && orderID != identity.OrderID {
				return transcriptRedactions{}, fmt.Errorf("perm id %d belongs to order ids %d and %d", identity.PermID, orderID, identity.OrderID)
			}
			permOrderIDs[identity.PermID] = identity.OrderID
		}
	}
	for _, identity := range identities {
		if identity.OCAGroup == "" {
			continue
		}
		if permID, err := strconv.ParseInt(identity.OCAGroup, 10, 64); err != nil || permOrderIDs[permID] == 0 {
			ocaOrdinal++
			if err := addToken(transcriptOCAGroup, identity.OCAGroup, "oca-", ocaOrdinal); err != nil {
				return transcriptRedactions{}, err
			}
		}
	}
	for permID, orderID := range permOrderIDs {
		digits := len(strconv.FormatInt(permID, 10))
		sanitized := int64(9)
		for range digits - 1 {
			sanitized *= 10
		}
		sanitized += orderID
		if len(strconv.FormatInt(sanitized, 10)) != digits {
			return transcriptRedactions{}, fmt.Errorf("perm id %d cannot preserve decimal width for order id %d", permID, orderID)
		}
		if err := addString(transcriptPermID, strconv.FormatInt(permID, 10), strconv.FormatInt(sanitized, 10)); err != nil {
			return transcriptRedactions{}, err
		}
		var from, to [binary.MaxVarintLen64]byte
		// permOrderIDs is populated only from positive broker identities, and
		// sanitized is constructed from positive decimal digits plus orderID.
		// #nosec G115 -- both signed values are proven positive above.
		fromN := binary.PutUvarint(from[:], uint64(permID))
		// #nosec G115 -- both signed values are proven positive above.
		toN := binary.PutUvarint(to[:], uint64(sanitized))
		if fromN != toN {
			return transcriptRedactions{}, fmt.Errorf("perm id %d -> %d changes protobuf varint width", permID, sanitized)
		}
		key := fmt.Sprintf("v:%d", permID)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			redactions.replacements = append(redactions.replacements, transcriptReplacement{
				kind: transcriptPermID, wire: transcriptVarint,
				from: append([]byte(nil), from[:fromN]...), to: append([]byte(nil), to[:toN]...),
			})
			redactions.changed = true
		}
	}
	return redactions, nil
}

func transcriptServerIdentities(replayEvents []capturelog.ReplayEvent) ([]transcriptDriverIdentity, error) {
	frameState := newCaptureFrameState()
	var identities []transcriptDriverIdentity
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if err := frameState.connect(event.Leg); err != nil {
				return nil, err
			}
		case capturelog.EventDisconnect:
			if err := frameState.disconnect(event.Leg); err != nil {
				return nil, err
			}
		case capturelog.ReplayEventFrame:
			payload, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				return nil, fmt.Errorf("decode replay frame: %w", err)
			}
			description, err := frameState.describe(event, payload)
			if err != nil {
				return nil, err
			}
			if !description.session || event.Direction != "server" {
				continue
			}
			messages, err := codec.DecodeBatch(description.serverVersion, payload)
			if err != nil {
				return nil, fmt.Errorf("decode server message %d: %w", description.msgID, err)
			}
			for _, message := range messages {
				switch message := message.(type) {
				case codec.ManagedAccounts:
					for _, account := range message.Accounts {
						identities = append(identities, transcriptDriverIdentity{Account: account})
					}
					continue
				case codec.OpenOrder:
					identity := transcriptOrderIdentity(message.OrderDetails)
					identity.OrderID = message.OrderID
					identities = append(identities, identity)
				case codec.CompletedOrder:
					identities = append(identities, transcriptOrderIdentity(message.OrderDetails))
				case codec.ExecutionDetail:
					identities = append(identities, transcriptDriverIdentity{
						Account:   message.Account,
						OrderID:   message.OrderID,
						OrderRef:  message.OrderRef,
						PermID:    parseTranscriptIdentityInt(message.PermID),
						ExecID:    message.ExecID,
						Submitter: message.Submitter,
					})
				}
			}
		}
	}
	return identities, nil
}

func transcriptOrderIdentity(details codec.OrderDetails) transcriptDriverIdentity {
	return transcriptDriverIdentity{
		Account:   details.Account,
		OrderID:   parseTranscriptIdentityInt(details.OrderID),
		OrderRef:  details.OrderRef,
		PermID:    parseTranscriptIdentityInt(details.PermID),
		OCAGroup:  details.OcaGroup,
		Submitter: details.Submitter,
	}
}

func parseTranscriptIdentityInt(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func lengthPreservingTranscriptToken(length int, prefix string, ordinal int) (string, error) {
	suffix := strconv.Itoa(ordinal)
	if len(prefix)+len(suffix) > length {
		return "", fmt.Errorf("cannot redact %d-byte value with %q token", length, prefix)
	}
	return prefix + strings.Repeat("0", length-len(prefix)-len(suffix)) + suffix, nil
}

type transcriptProtoRule struct {
	path      []protowire.Number
	kind      transcriptIdentityKind
	substring bool
}

func (r transcriptRedactions) applyFrame(direction string, description frameDescription, payload []byte) ([]byte, error) {
	if !description.session {
		return nil, errors.New("cannot sanitize a pre-session frame")
	}
	envelope, err := protocol.DecodeEnvelope(description.serverVersion, payload)
	if err != nil {
		return nil, err
	}
	if envelope.MsgID != description.msgID {
		return nil, fmt.Errorf("described msg_id %d differs from payload msg_id %d", description.msgID, envelope.MsgID)
	}

	var body []byte
	switch envelope.Encoding {
	case protocol.ProtobufBody:
		body, err = r.rewriteProto(envelope.Body, nil, transcriptProtoRules(direction, envelope.MsgID))
	case protocol.ClassicBody:
		body, err = r.rewriteClassic(direction, envelope.MsgID, envelope.Body)
	default:
		err = fmt.Errorf("unsupported body encoding %d", envelope.Encoding)
	}
	if err != nil {
		return nil, err
	}
	redacted := append([]byte(nil), payload[:4]...)
	return append(redacted, body...), nil
}

func transcriptProtoRules(direction string, msgID int) []transcriptProtoRule {
	rule := func(kind transcriptIdentityKind, path ...protowire.Number) transcriptProtoRule {
		return transcriptProtoRule{path: path, kind: kind}
	}
	substring := func(kind transcriptIdentityKind, path ...protowire.Number) transcriptProtoRule {
		return transcriptProtoRule{path: path, kind: kind, substring: true}
	}
	orderRules := func(prefix protowire.Number) []transcriptProtoRule {
		return []transcriptProtoRule{
			rule(transcriptPermID, prefix, 3),
			rule(transcriptAccount, prefix, 12),
			// The Gateway also uses the parent's permanent ID as an OCA group.
			rule(0, prefix, 27),
			rule(transcriptOrderRef, prefix, 28),
			rule(transcriptSubmitter, prefix, 137),
			rule(transcriptPermID, prefix, 121),
		}
	}

	if direction == "client" {
		switch msgID {
		case protocol.OutPlaceOrder:
			return orderRules(3)
		case protocol.OutReqExecutions:
			return []transcriptProtoRule{rule(transcriptAccount, 2, 2)}
		case protocol.OutExerciseOptions:
			return []transcriptProtoRule{rule(transcriptAccount, 5)}
		case protocol.OutReqAccountUpdates, protocol.OutReqAccountSummary,
			protocol.OutReqPositionsMulti, protocol.OutReqAccountUpdatesMulti,
			protocol.OutReqPnL, protocol.OutReqPnLSingle:
			return []transcriptProtoRule{rule(transcriptAccount, 2)}
		}
		return nil
	}
	if direction != "server" {
		return nil
	}

	switch msgID {
	case protocol.InManagedAccounts:
		return []transcriptProtoRule{substring(transcriptAccount, 1)}
	case protocol.InPositionData:
		return []transcriptProtoRule{rule(transcriptAccount, 1)}
	case protocol.InAccountSummary:
		return []transcriptProtoRule{rule(transcriptAccount, 2)}
	case protocol.InUpdateAccountValue:
		// AccountCode rows repeat the account in the generic value field.
		return []transcriptProtoRule{rule(transcriptAccount, 2), rule(transcriptAccount, 4)}
	case protocol.InUpdatePortfolio:
		return []transcriptProtoRule{rule(transcriptAccount, 8)}
	case protocol.InAccountDownloadEnd:
		return []transcriptProtoRule{rule(transcriptAccount, 1)}
	case protocol.InPositionMulti:
		return []transcriptProtoRule{rule(transcriptAccount, 2)}
	case protocol.InAccountUpdateMulti:
		// AccountCode rows repeat the account in the generic value field.
		return []transcriptProtoRule{rule(transcriptAccount, 2), rule(transcriptAccount, 5)}
	case protocol.InExecutionData:
		return []transcriptProtoRule{
			rule(transcriptExecID, 3, 2),
			rule(transcriptAccount, 3, 4),
			rule(transcriptPermID, 3, 9),
			rule(transcriptOrderRef, 3, 14),
			rule(transcriptSubmitter, 3, 20),
		}
	case protocol.InCommissionReport:
		return []transcriptProtoRule{rule(transcriptExecID, 1)}
	case protocol.InOpenOrder:
		rules := orderRules(3)
		return append(rules, rule(transcriptAccount, 4, 27, 1))
	case protocol.InCompletedOrder:
		return orderRules(2)
	case protocol.InOrderStatus:
		return []transcriptProtoRule{rule(transcriptPermID, 6)}
	case protocol.InOrderBound:
		return []transcriptProtoRule{rule(transcriptPermID, 1)}
	case protocol.InFamilyCodes:
		return []transcriptProtoRule{rule(transcriptAccount, 1, 1)}
	case protocol.InReceiveFA:
		return []transcriptProtoRule{substring(transcriptAccount, 2)}
	case protocol.InErrMsg:
		return []transcriptProtoRule{substring(0, 4), substring(0, 5)}
	default:
		return nil
	}
}

func (r transcriptRedactions) rewriteProto(body []byte, prefix []protowire.Number, rules []transcriptProtoRule) ([]byte, error) {
	redacted := make([]byte, 0, len(body))
	for len(body) > 0 {
		number, typ, tagLen := protowire.ConsumeTag(body)
		if tagLen < 0 {
			return nil, protowire.ParseError(tagLen)
		}
		redacted = append(redacted, body[:tagLen]...)
		body = body[tagLen:]
		path := append(append([]protowire.Number(nil), prefix...), number)
		exact, nested := protoRuleMatches(path, rules)

		switch typ {
		case protowire.VarintType:
			_, valueLen := protowire.ConsumeVarint(body)
			if valueLen < 0 {
				return nil, protowire.ParseError(valueLen)
			}
			value := body[:valueLen]
			if exact != nil {
				var err error
				value, err = r.rewriteIdentity(value, exact.kind, transcriptVarint, false)
				if err != nil {
					return nil, err
				}
			} else if replacement := r.matchingIdentity(value, transcriptVarint); replacement != nil {
				return nil, fmt.Errorf("identity %s appears at unapproved protobuf path %s", replacementName(replacement.kind), formatProtoPath(path))
			}
			redacted = append(redacted, value...)
			body = body[valueLen:]
		case protowire.BytesType:
			value, valueLen := protowire.ConsumeBytes(body)
			if valueLen < 0 {
				return nil, protowire.ParseError(valueLen)
			}
			prefixLen := valueLen - len(value)
			var err error
			switch {
			case exact != nil:
				value, err = r.rewriteIdentity(value, exact.kind, transcriptString, exact.substring)
			case nested:
				value, err = r.rewriteProto(value, path, rules)
			default:
				if replacement := r.containedStringIdentity(value); replacement != nil {
					return nil, fmt.Errorf("identity %s appears at unapproved protobuf path %s", replacementName(replacement.kind), formatProtoPath(path))
				}
			}
			if err != nil {
				return nil, err
			}
			if len(value) != valueLen-prefixLen {
				return nil, fmt.Errorf("redaction changed protobuf field %s length", formatProtoPath(path))
			}
			redacted = append(redacted, body[:prefixLen]...)
			redacted = append(redacted, value...)
			body = body[valueLen:]
		default:
			valueLen := protowire.ConsumeFieldValue(number, typ, body)
			if valueLen < 0 {
				return nil, protowire.ParseError(valueLen)
			}
			redacted = append(redacted, body[:valueLen]...)
			body = body[valueLen:]
		}
	}
	return redacted, nil
}

func protoRuleMatches(path []protowire.Number, rules []transcriptProtoRule) (*transcriptProtoRule, bool) {
	var nested bool
	for i := range rules {
		rule := &rules[i]
		if len(path) > len(rule.path) {
			continue
		}
		matches := true
		for j := range path {
			if path[j] != rule.path[j] {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		if len(path) == len(rule.path) {
			return rule, nested
		}
		nested = true
	}
	return nil, nested
}

func (r transcriptRedactions) rewriteIdentity(value []byte, kind transcriptIdentityKind, wire transcriptIdentityWire, substring bool) ([]byte, error) {
	redacted := append([]byte(nil), value...)
	for i := range r.replacements {
		replacement := &r.replacements[i]
		if replacement.wire != wire || kind != 0 && replacement.kind != kind {
			continue
		}
		if substring {
			redacted = bytes.ReplaceAll(redacted, replacement.from, replacement.to)
		} else if bytes.Equal(redacted, replacement.from) {
			redacted = append(redacted[:0], replacement.to...)
		}
	}
	if replacement := r.remainingIdentity(redacted, wire); replacement != nil {
		return nil, fmt.Errorf("identity %s remains in approved field", replacementName(replacement.kind))
	}
	return redacted, nil
}

func (r transcriptRedactions) remainingIdentity(value []byte, wire transcriptIdentityWire) *transcriptReplacement {
	if wire == transcriptVarint {
		return r.matchingIdentity(value, wire)
	}
	return r.containedIdentity(value, wire)
}

func (r transcriptRedactions) matchingIdentity(value []byte, wire transcriptIdentityWire) *transcriptReplacement {
	for i := range r.replacements {
		replacement := &r.replacements[i]
		if replacement.wire == wire && bytes.Equal(value, replacement.from) {
			return replacement
		}
	}
	return nil
}

func (r transcriptRedactions) containedStringIdentity(value []byte) *transcriptReplacement {
	return r.containedIdentity(value, transcriptString)
}

func (r transcriptRedactions) containedIdentity(value []byte, wire transcriptIdentityWire) *transcriptReplacement {
	for i := range r.replacements {
		replacement := &r.replacements[i]
		if replacement.wire == wire && bytes.Contains(value, replacement.from) {
			return replacement
		}
	}
	return nil
}

func (r transcriptRedactions) rewriteClassic(direction string, msgID int, body []byte) ([]byte, error) {
	if len(body) > 0 && body[len(body)-1] != 0 {
		return nil, wire.ErrMalformedFrame
	}
	fields := bytes.Split(body, []byte{0})
	if len(fields) > 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	for i, field := range fields {
		kind, substring, approved, err := classicTranscriptRule(direction, msgID, i, fields)
		if err != nil {
			return nil, err
		}
		if approved {
			field, err = r.rewriteIdentity(field, kind, transcriptString, substring)
			if err != nil {
				return nil, err
			}
			fields[i] = field
			continue
		}
		if replacement := r.containedStringIdentity(field); replacement != nil {
			return nil, fmt.Errorf("identity %s appears in unapproved classic field %d", replacementName(replacement.kind), i)
		}
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return append(bytes.Join(fields, []byte{0}), 0), nil
}

func classicTranscriptRule(direction string, msgID, index int, fields [][]byte) (transcriptIdentityKind, bool, bool, error) {
	if direction == "client" {
		switch msgID {
		case protocol.OutExerciseOptions:
			return transcriptAccount, false, index == 15, nil
		case protocol.OutReqPnL, protocol.OutReqPnLSingle:
			return transcriptAccount, false, index == 1, nil
		}
		return 0, false, false, nil
	}
	if direction != "server" {
		return 0, false, false, fmt.Errorf("invalid frame direction %q", direction)
	}
	switch msgID {
	case protocol.InFamilyCodes:
		if len(fields) == 0 {
			return 0, false, false, errors.New("family codes lacks count")
		}
		count, err := strconv.Atoi(string(fields[0]))
		if err != nil || count < 0 || len(fields) != 1+count*2 {
			return 0, false, false, fmt.Errorf("invalid family codes classic field count")
		}
		return transcriptAccount, false, index > 0 && index%2 == 1, nil
	case protocol.InReceiveFA:
		return transcriptAccount, true, index == 2, nil
	case protocol.InErrMsg:
		return 0, true, index == 3, nil
	default:
		return 0, false, false, nil
	}
}

func replacementName(kind transcriptIdentityKind) string {
	switch kind {
	case transcriptAccount:
		return "account"
	case transcriptOrderRef:
		return "order reference"
	case transcriptPermID:
		return "permanent ID"
	case transcriptOCAGroup:
		return "OCA group"
	case transcriptExecID:
		return "execution ID"
	case transcriptSubmitter:
		return "submitter"
	default:
		return "sensitive"
	}
}

func formatProtoPath(path []protowire.Number) string {
	parts := make([]string, len(path))
	for i, number := range path {
		parts[i] = strconv.Itoa(int(number))
	}
	return strings.Join(parts, ".")
}

type captureFrameState struct {
	legs map[int]captureLegState
}

type captureLegState struct {
	phase         captureLegPhase
	minVersion    int
	maxVersion    int
	serverVersion int
}

type captureLegPhase uint8

const (
	awaitVersionRange captureLegPhase = iota + 1
	awaitServerInfo
	sessionFrames
)

type frameDescription struct {
	label          string
	msgID          int
	encoding       string
	serverVersion  int
	connectionTime string
	session        bool
}

func (d frameDescription) messageID() string {
	if d.label != "" {
		return d.label
	}
	return strconv.Itoa(d.msgID)
}

func newCaptureFrameState() captureFrameState {
	return captureFrameState{legs: make(map[int]captureLegState)}
}

func (s *captureFrameState) connect(leg int) error {
	if _, ok := s.legs[leg]; ok {
		return fmt.Errorf("describe leg %d: duplicate connect", leg)
	}
	s.legs[leg] = captureLegState{phase: awaitVersionRange}
	return nil
}

func (s *captureFrameState) disconnect(leg int) error {
	if _, ok := s.legs[leg]; !ok {
		return fmt.Errorf("describe leg %d: disconnect before connect", leg)
	}
	delete(s.legs, leg)
	return nil
}

func (s *captureFrameState) describe(event capturelog.ReplayEvent, payload []byte) (frameDescription, error) {
	leg, ok := s.legs[event.Leg]
	if !ok {
		return frameDescription{}, fmt.Errorf("describe leg %d: frame before connect", event.Leg)
	}
	switch leg.phase {
	case awaitVersionRange:
		if event.Direction != "client" {
			return frameDescription{}, fmt.Errorf("describe leg %d: want client version range, got %s frame", event.Leg, event.Direction)
		}
		minVersion, maxVersion, err := decodeVersionRange(payload)
		if err != nil {
			return frameDescription{}, fmt.Errorf("describe leg %d version range: %w", event.Leg, err)
		}
		leg.phase = awaitServerInfo
		leg.minVersion = minVersion
		leg.maxVersion = maxVersion
		s.legs[event.Leg] = leg
		return frameDescription{label: "version_range", encoding: "pre_session"}, nil
	case awaitServerInfo:
		if event.Direction != "server" {
			return frameDescription{}, fmt.Errorf("describe leg %d: want server info, got %s frame", event.Leg, event.Direction)
		}
		info, err := codec.DecodeServerInfo(payload)
		if err != nil {
			return frameDescription{}, fmt.Errorf("describe leg %d server-info frame: %w", event.Leg, err)
		}
		if info.ServerVersion < protocol.SupportedMinServerVersion || info.ServerVersion > protocol.SupportedMaxServerVersion {
			return frameDescription{}, fmt.Errorf(
				"describe leg %d: server version %d outside supported range %d..%d",
				event.Leg,
				info.ServerVersion,
				protocol.SupportedMinServerVersion,
				protocol.SupportedMaxServerVersion,
			)
		}
		if info.ServerVersion < leg.minVersion || info.ServerVersion > leg.maxVersion {
			return frameDescription{}, fmt.Errorf(
				"describe leg %d: server version %d outside requested range %d..%d",
				event.Leg,
				info.ServerVersion,
				leg.minVersion,
				leg.maxVersion,
			)
		}
		leg.phase = sessionFrames
		leg.serverVersion = info.ServerVersion
		s.legs[event.Leg] = leg
		return frameDescription{label: "server_info", encoding: "pre_session", serverVersion: info.ServerVersion, connectionTime: info.ConnectionTime}, nil
	case sessionFrames:
		return describeSessionFrame(event, payload, leg.serverVersion)
	default:
		return frameDescription{}, fmt.Errorf("describe leg %d: invalid capture phase %d", event.Leg, leg.phase)
	}
}

func describeSessionFrame(event capturelog.ReplayEvent, payload []byte, serverVersion int) (frameDescription, error) {
	envelope, err := protocol.DecodeEnvelope(serverVersion, payload)
	if err != nil {
		return frameDescription{}, fmt.Errorf("describe leg %d %s frame: %w", event.Leg, event.Direction, err)
	}
	encoding := "classic"
	if envelope.Encoding == protocol.ProtobufBody {
		encoding = "protobuf"
	}
	return frameDescription{
		msgID:         envelope.MsgID,
		encoding:      encoding,
		serverVersion: serverVersion,
		session:       true,
	}, nil
}

func decodeVersionRange(payload []byte) (int, int, error) {
	value := string(payload)
	if !strings.HasPrefix(value, "v") {
		return 0, 0, fmt.Errorf("invalid payload %q", value)
	}
	minText, maxText, ok := strings.Cut(value[1:], "..")
	if !ok || minText == "" || maxText == "" {
		return 0, 0, fmt.Errorf("invalid payload %q", value)
	}
	minVersion, err := strconv.Atoi(minText)
	if err != nil || minVersion <= 0 {
		return 0, 0, fmt.Errorf("invalid minimum %q", minText)
	}
	maxVersion, err := strconv.Atoi(maxText)
	if err != nil || maxVersion <= 0 {
		return 0, 0, fmt.Errorf("invalid maximum %q", maxText)
	}
	if minVersion > maxVersion {
		return 0, 0, fmt.Errorf("minimum %d exceeds maximum %d", minVersion, maxVersion)
	}
	return minVersion, maxVersion, nil
}

func frameBytes(payload []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := wire.WriteFrame(&out, payload); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
