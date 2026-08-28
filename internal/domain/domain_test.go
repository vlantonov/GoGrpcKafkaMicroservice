package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
)

func TestNewOrder(t *testing.T) {
	t.Parallel()
	o := domain.NewOrder("widget", 3)
	assert.NotEmpty(t, o.ID)
	assert.Equal(t, "widget", o.Item)
	assert.Equal(t, uint32(3), o.Quantity)
	assert.Equal(t, domain.StatusPending, o.Status)
	assert.False(t, o.CreatedAt.IsZero())
}

func TestValidateCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		item     string
		quantity uint32
		wantErr  bool
	}{
		{"valid", "widget", 1, false},
		{"large quantity", "item", 999, false},
		{"empty item", "", 1, true},
		{"zero quantity", "widget", 0, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidateCreate(tc.item, tc.quantity)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTransition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		from    domain.Status
		to      domain.Status
		wantErr bool
	}{
		{"pending to confirmed", domain.StatusPending, domain.StatusConfirmed, false},
		{"pending to cancelled", domain.StatusPending, domain.StatusCancelled, false},
		{"confirmed to cancelled", domain.StatusConfirmed, domain.StatusCancelled, false},
		{"cancelled to pending - illegal", domain.StatusCancelled, domain.StatusPending, true},
		{"confirmed to pending - illegal", domain.StatusConfirmed, domain.StatusPending, true},
		{"pending to pending - illegal", domain.StatusPending, domain.StatusPending, true},
		{"cancelled to confirmed - illegal", domain.StatusCancelled, domain.StatusConfirmed, true},
		{"unspecified to pending - illegal", domain.StatusUnspecified, domain.StatusPending, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidateTransition(tc.from, tc.to)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
