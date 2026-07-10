package ibkr

import (
	"context"
	"errors"
	"testing"
)

// A nil-engine OptionsClient is intentional: every case must fail before the
// public API can enqueue actor work or write an exercise_options frame.
func TestExerciseRejectsInvalidMutatingRequestsBeforeEngine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   ExerciseAction
		quantity int
		field    string
		value    string
	}{
		{name: "missing action", action: 0, quantity: 1, field: "ExerciseAction", value: "0"},
		{name: "action above lapse", action: 3, quantity: 1, field: "ExerciseAction", value: "3"},
		{name: "zero quantity", action: Exercise, quantity: 0, field: "ExerciseQuantity", value: "0"},
		{name: "negative quantity", action: Lapse, quantity: -1, field: "ExerciseQuantity", value: "-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (OptionsClient{}).Exercise(context.Background(), ExerciseOptionsRequest{
				Contract:         Contract{ConID: 265598},
				ExerciseAction:   test.action,
				ExerciseQuantity: test.quantity,
			})
			validation, ok := errors.AsType[*ValidationError](err)
			if !ok {
				t.Fatalf("Exercise() error = %v, want *ValidationError", err)
			}
			if validation.Field != test.field || validation.Value != test.value {
				t.Fatalf("Exercise() validation = %#v, want field %q value %q", validation, test.field, test.value)
			}
		})
	}
}

// API 10.48's official Python exerciseOptions contract defines action 1 as
// exercise and action 2 as lapse. It sends account as provided without a
// client-side non-empty requirement.
func TestExerciseValidationAcceptsOfficialActionsWithoutAccount(t *testing.T) {
	t.Parallel()

	for _, action := range []ExerciseAction{Exercise, Lapse} {
		req := ExerciseOptionsRequest{
			Contract:         Contract{ConID: 265598},
			ExerciseAction:   action,
			ExerciseQuantity: 1,
		}
		if err := validateExerciseOptionsRequest(req); err != nil {
			t.Fatalf("validateExerciseOptionsRequest(%s) error = %v", action, err)
		}
	}
}
