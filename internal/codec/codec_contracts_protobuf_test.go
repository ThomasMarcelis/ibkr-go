package codec

import (
	"bytes"
	"testing"
)

func TestDeltaNeutralContractOfficialSchemaVector(t *testing.T) {
	t.Parallel()

	contract := Contract{
		ConID:        265598,
		DeltaNeutral: &DeltaNeutralContract{ConID: 265598, Delta: "0.5", Price: "314.5"},
	}
	got, err := encodeSharedContractProto(contract, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	// Official API 10.48.01 DeltaNeutralContract.proto: conId=1,
	// delta=2, price=3; Contract.deltaNeutralContract=17.
	want := decodeHex(t, "08fe9a108a011608fe9a1011000000000000e03f190000000000a87340")
	if !bytes.Equal(got, want) {
		t.Fatalf("encodeSharedContractProto() = %x, want official schema vector %x", got, want)
	}

	decoded, err := decodeSharedContractProto(got)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Contract.DeltaNeutral == nil ||
		decoded.Contract.DeltaNeutral.ConID != 265598 ||
		decoded.Contract.DeltaNeutral.Delta != "0.5" ||
		decoded.Contract.DeltaNeutral.Price != "314.5" {
		t.Fatalf("decoded delta-neutral contract = %+v", decoded.Contract.DeltaNeutral)
	}
}

func TestDeltaNeutralContractProtoOmittedScalarsDefaultZero(t *testing.T) {
	t.Parallel()

	// API 10.48.01 uses proto3 optional doubles for delta and price. Omitting
	// either tag therefore decodes as zero; only conId is present here.
	decoded, err := decodeSharedContractProto(decodeHex(t, "8a010408fe9a10"))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Contract.DeltaNeutral == nil ||
		decoded.Contract.DeltaNeutral.ConID != 265598 ||
		decoded.Contract.DeltaNeutral.Delta != "0" ||
		decoded.Contract.DeltaNeutral.Price != "0" {
		t.Fatalf("decoded omitted delta-neutral scalars = %+v", decoded.Contract.DeltaNeutral)
	}
}
