// SPDX-License-Identifier: Apache-2.0

package log4j2config

import (
	"encoding/xml"
	"io"
	"strings"

	"github.com/hashgraph/solo-operator/internal/errx"
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
			return errx.WithHints(
				errx.WithReason(errValidation.New("log4j2 XML has no root element"), ReasonMalformedXML),
				"Provide a <Configuration> root element in the log4j2 XML",
			)
		}
		if err != nil {
			return errx.WithHints(
				errx.WithReason(errValidation.Wrap(err, "log4j2 XML is not well-formed"), ReasonMalformedXML),
				"Fix the XML syntax so the document parses",
			)
		}
		if se, ok := tok.(xml.StartElement); ok {
			root = se
			break
		}
	}

	if root.Name.Local != configRootElement {
		return errx.WithHints(
			errx.WithReason(errValidation.New("log4j2 XML root element must be <Configuration>, got <%s>", root.Name.Local), ReasonWrongRootElement),
			"Use <Configuration> as the root element of the log4j2 XML",
		)
	}

	if root.Name.Space != "" && root.Name.Space != log4jNamespace {
		return errx.WithHints(
			errx.WithReason(errValidation.New("log4j2 XML root element has unexpected namespace %q (expected empty or %q)", root.Name.Space, log4jNamespace), ReasonWrongNamespace),
			"Use no namespace or "+log4jNamespace+" on the <Configuration> element",
		)
	}

	// Consume the rest of the document to catch any remaining well-formedness errors.
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errx.WithHints(
				errx.WithReason(errValidation.Wrap(err, "log4j2 XML is not well-formed"), ReasonMalformedXML),
				"Fix the XML syntax so the document parses",
			)
		}
	}

	return nil
}
