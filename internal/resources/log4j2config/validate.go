// SPDX-License-Identifier: Apache-2.0

package log4j2config

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	log4jNamespace    = "https://logging.apache.org/xml/ns"
	configRootElement = "Configuration"
)

// ValidateContent rejects malformed XML and wrong root elements.
// Accepts no-namespace or the official Log4j2 namespace on <Configuration>.
func ValidateContent(content string) error {
	decoder := xml.NewDecoder(strings.NewReader(content))

	// Advance to the first start element.
	var root xml.StartElement
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return fmt.Errorf("log4j2 XML has no root element")
		}
		if err != nil {
			return fmt.Errorf("log4j2 XML is not well-formed: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			root = se
			break
		}
	}

	if root.Name.Local != configRootElement {
		return fmt.Errorf("log4j2 XML root element must be <Configuration>, got <%s>", root.Name.Local)
	}

	if root.Name.Space != "" && root.Name.Space != log4jNamespace {
		return fmt.Errorf("log4j2 XML root element has unexpected namespace %q (expected empty or %q)", root.Name.Space, log4jNamespace)
	}

	// Consume the rest of the document to catch any remaining well-formedness errors.
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("log4j2 XML is not well-formed: %w", err)
		}
	}

	return nil
}
