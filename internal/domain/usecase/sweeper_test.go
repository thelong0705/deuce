package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/usecase"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

func TestSweeperReleaseLapsedHolds(t *testing.T) {
	boom := errors.New("boom")
	asOf := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		setup func(holds *mocks.MockHoldReleaser)
		// wantReleased is how many slots the caller is told about
		wantReleased int
		wantErr      error
	}{
		{
			name: "reports what it put back",
			setup: func(holds *mocks.MockHoldReleaser) {
				holds.EXPECT().ReleaseLapsedHolds(mock.Anything, asOf).Return(3, nil).Once()
			},
			wantReleased: 3,
		},
		{
			name: "nothing overdue is not an error",
			setup: func(holds *mocks.MockHoldReleaser) {
				holds.EXPECT().ReleaseLapsedHolds(mock.Anything, asOf).Return(0, nil).Once()
			},
		},
		{
			name: "propagates a storage failure",
			setup: func(holds *mocks.MockHoldReleaser) {
				holds.EXPECT().ReleaseLapsedHolds(mock.Anything, asOf).Return(0, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holds := mocks.NewMockHoldReleaser(t)
			tt.setup(holds)

			released, err := usecase.NewSweeper(holds).
				ReleaseLapsedHolds(context.Background(), asOf)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantReleased, released)
		})
	}
}
