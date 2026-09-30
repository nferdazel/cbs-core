package config

import "testing"

// Di production, kata sandi DB default (atau kosong) harus menolak start; di
// non-production keduanya sah untuk pengembangan lokal.
func TestProductionDBPasswordFatal(t *testing.T) {
	cases := []struct {
		nama        string
		environment string
		password    string
		inginFatal  bool
	}{
		{"production default", "production", defaultDBPassword, true},
		{"production kosong", "production", "", true},
		{"production diisi", "production", "s3cret-db-pass", false},
		{"development default", "development", defaultDBPassword, false},
		{"development kosong", "development", "", false},
		{"development diisi", "development", "apa-saja", false},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			msg := productionDBPasswordFatal(c.environment, c.password)
			if (msg != "") != c.inginFatal {
				t.Fatalf("environment=%q password=%q: fatal=%v (pesan %q), ingin fatal=%v",
					c.environment, c.password, msg != "", msg, c.inginFatal)
			}
		})
	}
}
