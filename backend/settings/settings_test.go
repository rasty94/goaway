package settings

import "testing"

func TestUpdateKeepsClusterSecret(t *testing.T) {
	t.Chdir(t.TempDir())
	config := &Config{}
	config.HighAvailability.Secret = "keep-me"
	incoming := Config{}
	incoming.HighAvailability.Mode = "replica"
	config.Update(incoming)
	if config.HighAvailability.Secret != "keep-me" || config.HighAvailability.Mode != "replica" {
		t.Fatalf("secret lost or mode not applied: %+v", config.HighAvailability)
	}
}
