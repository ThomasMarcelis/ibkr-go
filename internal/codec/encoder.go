package codec

import (
	"math"
	"strconv"
)

type fieldWriter struct {
	fields []string
}

func (w *fieldWriter) WriteInt(v int) {
	w.fields = append(w.fields, strconv.Itoa(v))
}

func (w *fieldWriter) WriteInt64(v int64) {
	w.fields = append(w.fields, strconv.FormatInt(v, 10))
}

func (w *fieldWriter) WriteFloat(v float64) {
	w.fields = append(w.fields, strconv.FormatFloat(v, 'f', -1, 64))
}

// WriteMaxFloat writes an empty string for math.MaxFloat64 (the TWS sentinel
// for "no value"), otherwise formats the float normally.
func (w *fieldWriter) WriteMaxFloat(v float64) {
	if v >= math.MaxFloat64 {
		w.fields = append(w.fields, "")
		return
	}
	w.WriteFloat(v)
}

// WriteMaxInt writes an empty string for math.MaxInt32 (the TWS sentinel),
// otherwise formats the int normally.
func (w *fieldWriter) WriteMaxInt(v int) {
	if v == math.MaxInt32 {
		w.fields = append(w.fields, "")
		return
	}
	w.WriteInt(v)
}

func (w *fieldWriter) WriteString(v string) {
	w.fields = append(w.fields, v)
}

func (w *fieldWriter) WriteBool(v bool) {
	if v {
		w.fields = append(w.fields, "1")
	} else {
		w.fields = append(w.fields, "0")
	}
}

// WriteDecimal writes a raw decimal string (passthrough, no float conversion).
func (w *fieldWriter) WriteDecimal(v string) {
	w.fields = append(w.fields, v)
}

func (w *fieldWriter) Fields() []string {
	return w.fields
}
