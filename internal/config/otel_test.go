package config

import "testing"

func TestOTelDefaultsAndValidate(t *testing.T) {
	var o OTel
	if o.Enabled() || o.ProtocolOrDefault() != "grpc" || o.SampleOrDefault() != 1 {
		t.Errorf("zero value: %+v", o)
	}
	if err := o.Validate(); err != nil {
		t.Errorf("zero value must validate: %v", err)
	}
	o = OTel{Endpoint: "c:4318", Protocol: "http", Sample: 0.25}
	if !o.Enabled() || o.ProtocolOrDefault() != "http" || o.SampleOrDefault() != 0.25 {
		t.Errorf("explicit: %+v", o)
	}
	for _, bad := range []OTel{{Protocol: "udp"}, {Sample: 1.5}, {Sample: -0.1}} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%+v must be rejected", bad)
		}
	}
}
