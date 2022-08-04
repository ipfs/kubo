package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/multiformats/go-multihash"
)

func TestProvideManyWrapper_ProvideMany(t *testing.T) {
	type fields struct {
		pms []ProvideMany
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		ready   bool
	}{
		{
			name: "one provider",
			fields: fields{
				pms: []ProvideMany{
					newDummyProvideMany(true, false),
				},
			},
			wantErr: false,
			ready:   true,
		},
		{
			name: "two providers, no errors and ready",
			fields: fields{
				pms: []ProvideMany{
					newDummyProvideMany(true, false),
					newDummyProvideMany(true, false),
				},
			},
			wantErr: false,
			ready:   true,
		},
		{
			name: "two providers, no ready, no error",
			fields: fields{
				pms: []ProvideMany{
					newDummyProvideMany(true, false),
					newDummyProvideMany(false, false),
				},
			},
			wantErr: false,
			ready:   false,
		},
		{
			name: "two providers, no ready, and one erroing",
			fields: fields{
				pms: []ProvideMany{
					newDummyProvideMany(true, false),
					newDummyProvideMany(false, true),
				},
			},
			wantErr: true,
			ready:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pmw := &ProvideManyWrapper{
				pms: tt.fields.pms,
			}
			if err := pmw.ProvideMany(context.Background(), nil); (err != nil) != tt.wantErr {
				t.Errorf("ProvideManyWrapper.ProvideMany() error = %v, wantErr %v", err, tt.wantErr)
			}

			if ready := pmw.Ready(); ready != tt.ready {
				t.Errorf("ProvideManyWrapper.Ready() unexpected output = %v, want %v", ready, tt.ready)
			}
		})
	}
}

func newDummyProvideMany(ready, failProviding bool) *dummyProvideMany {
	return &dummyProvideMany{
		ready:         ready,
		failProviding: failProviding,
	}
}

type dummyProvideMany struct {
	ready, failProviding bool
}

func (dpm *dummyProvideMany) ProvideMany(ctx context.Context, keys []multihash.Multihash) error {
	if dpm.failProviding {
		return errors.New("error providing many")
	}

	return nil
}
func (dpm *dummyProvideMany) Ready() bool {
	return dpm.ready
}
