package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/offen/docker-volume-backup/internal/errwrap"
	"github.com/traefik/paerser/env"
)

type envVarLookup struct {
	ok    bool
	key   string
	value string
}

type envVarMapping struct {
	from string
	to   string
}

func mapLegacyVariables() (func() error, error) {
	lookups := []envVarLookup{}
	unset := func() error {
		for _, lookup := range lookups {
			if !lookup.ok {
				if err := os.Unsetenv(lookup.key); err != nil {
					return errwrap.Wrap(err, fmt.Sprintf("error unsetting env var %s", lookup.key))
				}
				continue
			}
			if err := os.Setenv(lookup.key, lookup.value); err != nil {
				return errwrap.Wrap(err, fmt.Sprintf("error setting back env var %s", lookup.key))
			}
		}
		return nil
	}

	mapping := []envVarMapping{
		{from: "AWS_ACCESS_KEY_ID", to: "OFFEN_STORAGE_AWS_ACCESSKEYID"},
		{from: "AWS_SECRET_ACCESS_KEY", to: "OFFEN_STORAGE_AWS_SECRETACCESSKEY"},
		{from: "AWS_SECRET_ACCESS_KEY_FILE", to: "FILE__OFFEN_STORAGE_AWS_SECRETACCESSKEY"},
		{from: "AWS_ENDPOINT", to: "OFFEN_STORAGE_AWS_ENDPOINT"},
		{from: "AWS_ENDPOINT_PROTO", to: "OFFEN_STORAGE_AWS_ENDPOINTPROTO"},
		{from: "AWS_S3_BUCKET_NAME", to: "OFFEN_STORAGE_AWS_BUCKETNAME"},
		{from: "BACKUP_FILENAME_EXPAND", to: "OFFEN_BACKUP_FILENAMEEXPAND"},
		{from: "BACKUP_FILENAME", to: "OFFEN_BACKUP_FILENAME"},
		{from: "BACKUP_CRON_EXPRESSION", to: "OFFEN_BACKUP_CRONEXPRESSION"},
		{from: "BACKUP_RETENTION_DAYS", to: "OFFEN_BACKUP_RETENTIONDAYS"},
		{from: "BACKUP_PRUNING_LEEWAY", to: "OFFEN_BACKUP_PRUNINGLEEWAY"},
		{from: "BACKUP_PRUNING_PREFIX", to: "OFFEN_BACKUP_PRUNINGPREFIX"},
	}

	for _, value := range mapping {
		legacy, legacySet := os.LookupEnv(value.from)
		lookups = append(lookups, envVarLookup{ok: legacySet, key: value.from, value: legacy})
		if legacySet {
			os.Unsetenv(value.from)

			new, newSet := os.LookupEnv(value.to)
			if newSet {
				return nil, fmt.Errorf("environment variable %s is set, while it's legacy option %s is also set", value.to, value.from)
			}
			lookups = append(lookups, envVarLookup{ok: newSet, key: value.from, value: new})
			os.Setenv(value.to, legacy)
		}
	}

	return unset, nil
}

func mapEnvironmentFiles() (func() error, error) {
	lookups := []envVarLookup{}
	unset := func() error {
		for _, lookup := range lookups {
			if !lookup.ok {
				if err := os.Unsetenv(lookup.key); err != nil {
					return errwrap.Wrap(err, fmt.Sprintf("error unsetting env var %s", lookup.key))
				}
				continue
			}
			if err := os.Setenv(lookup.key, lookup.value); err != nil {
				return errwrap.Wrap(err, fmt.Sprintf("error setting back env var %s", lookup.key))
			}
		}
		return nil
	}

	c := &Config{}
	vars := env.FindPrefixedEnvVars(os.Environ(), "FILE__OFFEN_", c)

	for _, evr := range vars {
		k, v, _ := strings.Cut(evr, "=")
		err := os.Unsetenv(k)
		if err != nil {
			return nil, errwrap.Wrap(err, fmt.Sprintf("unable to unset environment variable %s", k))
		}
		lookups = append(lookups, envVarLookup{ok: true, key: k, value: v})

		newVar := strings.TrimPrefix(k, "FILE__")
		new, newSet := os.LookupEnv(newVar)
		if newSet {
			return nil, fmt.Errorf("environment variable %s is set, while file option %s is also set", newVar, k)
		}

		contents, err := os.ReadFile(v)
		if err != nil {
			return nil, errwrap.Wrap(err, fmt.Sprintf("unable to read variable from file %s", v))
		}

		err = os.Setenv(newVar, string(contents))
		if err != nil {
			return nil, errwrap.Wrap(err, fmt.Sprintf("unable to set environment variable %s", newVar))
		}
		lookups = append(lookups, envVarLookup{ok: newSet, key: newVar, value: new})
	}

	return unset, nil
}

// applyEnv sets the values in `additionalEnvVars` as environment variables.
// It returns a function that reverts all values that have been set to its
// previous state.
func ApplyEnv() (func() error, error) {
	lookups := []envVarLookup{}

	unset := func() error {
		for _, lookup := range lookups {
			if !lookup.ok {
				if err := os.Unsetenv(lookup.key); err != nil {
					return errwrap.Wrap(err, fmt.Sprintf("error unsetting env var %s", lookup.key))
				}
				continue
			}
			if err := os.Setenv(lookup.key, lookup.value); err != nil {
				return errwrap.Wrap(err, fmt.Sprintf("error setting back env var %s", lookup.key))
			}
		}
		return nil
	}

	//	for key, value := range c.additionalEnvVars {
	//		current, ok := os.LookupEnv(key)
	//		lookups = append(lookups, envVarLookup{ok: ok, key: key, value: current})
	//		if err := os.Setenv(key, value); err != nil {
	//			return unset, errwrap.Wrap(err, "error setting env var")
	//		}
	//	}
	return unset, nil
}
