package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

const schemaFileName = "skill-manifest-v1.schema.json"

var (
	schemaOnce  sync.Once
	schemaBytes []byte
	schemaErr   error
)

func validate(data []byte) error {
	schema, err := loadSchemaBytes()
	if err != nil {
		return err
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schema),
		gojsonschema.NewBytesLoader(data),
	)
	if err != nil {
		return fmt.Errorf("validate manifest against schema: %w", err)
	}

	if result.Valid() {
		return nil
	}

	messages := make([]string, 0, len(result.Errors()))
	for _, validationError := range result.Errors() {
		messages = append(messages, validationError.String())
	}

	return fmt.Errorf("manifest failed schema validation: %s", strings.Join(messages, "; "))
}

func loadSchemaBytes() ([]byte, error) {
	schemaOnce.Do(func() {
		schemaPath, err := schemaPath()
		if err != nil {
			schemaErr = err
			return
		}

		schemaBytes, schemaErr = os.ReadFile(schemaPath)
		if schemaErr != nil {
			schemaErr = fmt.Errorf("read manifest schema %q: %w", schemaPath, schemaErr)
		}
	})

	return schemaBytes, schemaErr
}

func schemaPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve schema path: runtime.Caller(0) failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../../../schemas", schemaFileName)), nil
}
