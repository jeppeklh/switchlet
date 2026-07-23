package editor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func replaceConnectionString(contents []byte, connectionName string, replacementValue string) ([]byte, error) {
	rootObject, connectionStringsObject, err := parseConnectionStringTarget(contents, connectionName)
	if err != nil {
		return nil, err
	}

	connectionStringsObject[connectionName] = replacementValue

	updatedContents, err := json.MarshalIndent(rootObject, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize updated JSON: %w", err)
	}

	return append(updatedContents, '\n'), nil
}

func parseConnectionStringTarget(contents []byte, connectionName string) (map[string]any, map[string]any, error) {
	rootObject, connectionStringsObject, err := parseConnectionStringsObject(contents)
	if err != nil {
		return nil, nil, err
	}

	connectionValue, ok := connectionStringsObject[connectionName]
	if !ok {
		return nil, nil, fmt.Errorf("does not contain connection string %q", connectionName)
	}
	if _, ok := connectionValue.(string); !ok {
		return nil, nil, fmt.Errorf("connection string %q must be a string", connectionName)
	}

	return rootObject, connectionStringsObject, nil
}

func parseConnectionStringsObject(contents []byte) (map[string]any, map[string]any, error) {
	decodedDocument, err := parseJSONDocument(contents)
	if err != nil {
		return nil, nil, fmt.Errorf("contains invalid JSON: %w", err)
	}

	rootObject, ok := decodedDocument.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("must contain a JSON object at the root")
	}

	connectionStringsValue, ok := rootObject["ConnectionStrings"]
	if !ok {
		return nil, nil, fmt.Errorf("must contain a ConnectionStrings object")
	}

	connectionStringsObject, ok := connectionStringsValue.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("ConnectionStrings must be an object")
	}

	return rootObject, connectionStringsObject, nil
}

func parseJSONDocument(contents []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()

	var decodedDocument any
	if err := decoder.Decode(&decodedDocument); err != nil {
		return nil, err
	}

	var extraValue any
	if err := decoder.Decode(&extraValue); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values are not allowed")
		}

		return nil, err
	}

	return decodedDocument, nil
}
