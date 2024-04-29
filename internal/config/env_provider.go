package config

import (
	"fmt"
	"os"

	"github.com/offen/docker-volume-backup/internal/errwrap"
	"github.com/traefik/paerser/env"
)

func loadConfigFromEnvVars() (*Config, error) {
	c := &Config{}
	c.SetDefaults()

	unsetMap, err := mapLegacyVariables()
	if err != nil {
		return nil, errwrap.Wrap(err, "failed to map legacy variables")
	}
	defer unsetMap()

	unsetFiles, err := mapEnvironmentFiles()
	if err != nil {
		return nil, errwrap.Wrap(err, "failed to map variables from files")
	}
	defer unsetFiles()

	vars := env.FindPrefixedEnvVars(os.Environ(), "OFFEN_", c)

	if len(vars) == 0 {
		return nil, fmt.Errorf("failed to load environment variables")
	}

	if err := env.Decode(vars, "OFFEN_", c); err != nil {
		return nil, errwrap.Wrap(err, "failed to decode configuration from environment variables")
	}

	c.Source = "from environment"
	return c, nil
}
