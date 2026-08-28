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

func TestContractDetailsSettlementMethodProto225LiveFrames(t *testing.T) {
	t.Parallel()

	// API 10.50.01 ContractDetails.proto adds settlementMethod at field 65.
	// These exact sv225 frames were captured from the live Gateway on
	// 2026-08-28: an AAPL option reports physical delivery and an ES future
	// reports cash settlement.
	for _, test := range []struct {
		name       string
		frame      string
		settlement string
		hash       string
	}{
		{
			name:       "option",
			frame:      "H4sIAAAAAAACA6VTP28TMRS/S0pVOkEllooh6gTSA5wrVE1Y6rOdiznf2dhOSbq1VSSKqg4tQpQPwMDE3AkxVmIBiQ/AyEYXvgczQjz7EjohIXG63/PvPfveP79LkvbnJEkullorL5e+fX/9ob2yQKlRq21t/NrNjGQbZDPb7HQ3+oTg2xm5e2L35Pn0+Oh2Ep53R1tZynqRJ5Ot/IqrqPW2PXJ850bw1Ok0Lhgh2f0HhJC9GOA8XZo7X/20uBxtKwvkLumu/mxR5uW28LICyvF9NHIeqBI2yEKjUJoB1TXQ7YJp3MypK4UHpmsehbZcWOB0Alygs0Zy6RoiNIegDHQJBfVQeIbgCA9DyUEyARIjKOlBVR4qXKsS4RXUGLDKodZ1TEUzCtqImintBJiQkxFFJfm2VpGVfsaMrWbM2UFkmLiu1QSMxYBsWIIVKsAwrwcuMOdLcAwrb6TFSp3ciWdjm50vwNXUYLxmxSTDGhw5bwJC/t5SqRoZSmrI3B6Kw3SwDU+G1MvB2qvoGmglxsByjVUN1RgMlicRuR5jt72DmjpOH+sqnslmaj7GZtExFKKStQTBCzQIy0YWyxTUKhCoUsXRWo1B5iOHcwaOGjOUVtxKya+za/2rqCrRkTXLF5sh2Vu+nLtnD+eD0ye9dXLnj9bdIGS+1+vHG+EzfZ3M9OP/+fht2jpL46S+T9t4N+dpuZ5Bl/T+itn2P536ePlHfEnT7lfEBeJHOyVvFq6bp6cnB/u7hx0+PTx4MT0+/Q0NJdQTugMAAA==",
			settlement: "Physical Delivery",
			hash:       "86b72d3b5bfbb3ff401753020627152b4b03f9af2d99cb20118b3455d435162f",
		},
		{
			name:       "future",
			frame:      "H4sIAAAAAAACA41SPW/TQBj21QmKMjBkjBiiDgikt3B2VCcxC+e7q3PE9hnfpUAHpFaqRFHbofAr+AkwgJhQ2ZiYERNjJ2Yk/gASO68/aJwuYPn9eO45P++j8zmOmzmOc9Ejg6e9z19//CaDDWmG7s7Sbt7wqR94vjcd0Wk4piGlo6W5yw9PX57tH992mmdWF3U/cnkqC3dpxF5Hmr3gAIXOSe+vyPBnt48rgw69428P320wbtWutCoFJvB9sDQWWCKLMscaU6I5sN2YayQiZhYSi8z4PH8MXGeiSroQsgDBnoCQKFhnoUzdSC0gZhZiyzGwtxbmSoDiEhSqJ8pCkmLYOaTYp4sq8kJjtQlkODyNINNZZUlzBrmMKxNgOJqtc4EGTcbyFKWruqhxIRMwNi+jnIKlUrYFU0mdSwN1kzZNaePRnFm1s9nH44SHUTn6FqEfvr/thtfl1snR6dHI3MxH25RG1+rDPeiv/svzN6RcpFN/EnoTSrcaNA29gNJ7DZqFPNFGigbjz23tHXvtvSW65Ga0zVWoxflrnL/GjSvu7CO59FOO/V9369hrfzu+6mjFXXXrr3H/dvua4IV9T1yViXPSDSYQTD6tLvQXQrxvGBcYv1xCX3U6fP/Fsz9Ug+5GUgMAAA==",
			settlement: "Cash",
			hash:       "f9092f15375af724a723b32907a717d5ae7ea2dd84bba2e67b2381d1b3a4714e",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame := decodeGzipBase64(t, test.frame)
			decoded, err := Decode(225, frame[4:])
			if err != nil {
				t.Fatal(err)
			}
			details, ok := decoded.(ContractDetails)
			if !ok {
				t.Fatalf("Decode() = %T, want ContractDetails", decoded)
			}
			if details.SettlementMethod != test.settlement {
				t.Fatalf("SettlementMethod = %q, want %q; capture events sha256 %s", details.SettlementMethod, test.settlement, test.hash)
			}
		})
	}
}
