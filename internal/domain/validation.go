package domain

import "fmt"

// ValidateCreate checks that item is non-empty and quantity is at least 1.
func ValidateCreate(item string, quantity uint32) error {
	if item == "" {
		return fmt.Errorf("item must not be empty")
	}
	if quantity == 0 {
		return fmt.Errorf("quantity must be at least 1")
	}
	return nil
}

// ValidateTransition enforces the allowed order status state machine.
// Legal transitions: Pending→Confirmed, Pending→Cancelled, Confirmed→Cancelled.
func ValidateTransition(current, next Status) error {
	allowed := map[Status][]Status{
		StatusPending:   {StatusConfirmed, StatusCancelled},
		StatusConfirmed: {StatusCancelled},
	}
	for _, s := range allowed[current] {
		if s == next {
			return nil
		}
	}
	return fmt.Errorf("transition from %v to %v is not allowed", current, next)
}
