//go:build !remote && (linux || freebsd)

package abi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/secrets"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func Test_secretToReport(t *testing.T) {
	type args struct {
		secret     secrets.Secret
		secretData string
	}
	tests := []struct {
		name string
		args args
		want *entities.SecretInfoReport
	}{
		{
			name: "test secretToReport",
			args: args{
				secret: secrets.Secret{
					Name: "test-name",
					ID:   "test-id",
					Labels: map[string]string{
						"test-label": "test-value",
					},
					CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC),
					Driver:    "test-driver",
					DriverOptions: map[string]string{
						"test-driver-option": "test-value",
					},
				},
				secretData: "test-secret-data",
			},
			want: &entities.SecretInfoReport{
				ID:        "test-id",
				CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC),
				Spec: entities.SecretSpec{
					Name: "test-name",
					Driver: entities.SecretDriverSpec{
						Name:    "test-driver",
						Options: map[string]string{"test-driver-option": "test-value"},
					},
					Labels: map[string]string{"test-label": "test-value"},
				},
				SecretData: "test-secret-data",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, secretToReportWithData(tt.args.secret, tt.args.secretData), "secretToReport(%v)", tt.args.secret)
		})
	}
}
