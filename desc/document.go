package desc

// Document is the envelope a described command tree is published in. See
// the package doc for the version policy SchemaVersion carries.
type Document struct {
	// SchemaVersion identifies the shape of this document. See the
	// package doc.
	SchemaVersion int `json:"schemaVersion"`

	// Command is the root of the described tree.
	Command *Command `json:"command"`
}

// NewDocument returns a Document wrapping cmd at the current schema
// version.
func NewDocument(cmd *Command) *Document {
	return &Document{
		SchemaVersion: 1,
		Command:       cmd,
	}
}
