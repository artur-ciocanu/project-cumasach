package manifest

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed skill-manifest-v1.schema.json
var schemaBytes []byte

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
	if len(schemaBytes) == 0 {
		return nil, fmt.Errorf("embedded manifest schema is empty")
	}

	return schemaBytes, nil
}
