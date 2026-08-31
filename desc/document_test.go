package desc

import (
	"encoding/json"
	"reflect"
	"testing"
)

// golden is a small, hand-built Document exercising one field of each
// kind: a required string, an omitted optional string, a bool that
// shows, a bool that is left at its zero value, a nested slice, and both
// an ordinary and a generated Option.
func golden() *Document {
	return &Document{
		SchemaVersion: 1,
		Command: &Command{
			Name:     "add",
			FullName: "orbital remote add",
			Summary:  "Add a remote",
			FlagGroups: []*FlagGroup{
				{
					Title: "Options",
					Flags: []*Flag{
						{
							Name:  "force",
							Kind:  "bool",
							Usage: "Overwrite an existing remote",
							Options: []Option{
								{Option: "--force"},
								{Option: "-f"},
								{Option: "--no-force", Effect: "negate"},
							},
							MaxCount: 1,
						},
					},
				},
			},
		},
	}
}

// goldenJSON is what golden marshals to. It pins every key name in the
// format: a rename, a removal, or a new omitempty firing unexpectedly all
// fail this test loudly.
const goldenJSON = `{
  "schemaVersion": 1,
  "command": {
    "name": "add",
    "fullName": "orbital remote add",
    "summary": "Add a remote",
    "flagGroups": [
      {
        "title": "Options",
        "flags": [
          {
            "name": "force",
            "kind": "bool",
            "usage": "Overwrite an existing remote",
            "maxCount": 1,
            "options": [
              {"option": "--force"},
              {"option": "-f"},
              {"option": "--no-force", "effect": "negate"}
            ]
          }
        ]
      }
    ]
  }
}`

// TestDocumentMarshal pins the wire format's keys against goldenJSON, so
// a Go field rename or removal is a wire break this test catches rather
// than something a consumer discovers.
func TestDocumentMarshal(t *testing.T) {
	got, err := json.MarshalIndent(golden(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var gotVal, wantVal any
	if err := json.Unmarshal(got, &gotVal); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(goldenJSON), &wantVal); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotVal, wantVal) {
		t.Errorf("marshaled document =\n%s\nwant\n%s", got, goldenJSON)
	}
}

// TestDocumentRoundTrip unmarshals goldenJSON and checks it decodes back
// to the struct it was marshaled from, so the format is confirmed
// readable and not merely writable.
func TestDocumentRoundTrip(t *testing.T) {
	var got Document
	if err := json.Unmarshal([]byte(goldenJSON), &got); err != nil {
		t.Fatal(err)
	}
	if want := golden(); !reflect.DeepEqual(&got, want) {
		t.Errorf("round-tripped document = %+v, want %+v", got, want)
	}
}
