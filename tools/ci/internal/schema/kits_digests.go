// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

// kitSpecDigests pins the copy of the kit schema of sbx: the SHA-256 of
// each file of third_party/sandbox-kit-spec/schema/, by its path below
// it. The check holds the tree's copy to this table, so a file edited,
// added or removed fails unless the table changes with it, in a change
// that tools/ci/internal/schema/** puts under the checks surface
// (.github/ask-first.yaml). The digests are those of the files at
// kitSpecTag with the one change the README there names.
var kitSpecDigests = map[string]string{
	"capabilities/com.docker.sandbox/agent-context@1.schema.json":  "f89642b266f1622573236e1c7623ad264c43efbc885107854b4a97343e98ba2a",
	"capabilities/com.docker.sandbox/agent-sessions@1.schema.json": "8223514d36f15bd200a9938a15f020148054eb31a8acc05c1a208c509d15ecac",
	"capabilities/com.docker.sandbox/agent-skill@1.schema.json":    "91dd731450f0d0f1a3494106fc336f478e606b0ff0201bc669aa1d771c662298",
	"capabilities/com.docker.sandbox/agent-skills@1.schema.json":   "1ac076a3b8732c677795fd02e4c8a498324654c46acc92796f9cc07721928ddb",
	"capabilities/com.docker.sandbox/credential@1.schema.json":     "08fb7fcf790c6a90c94667b75f6734a3f6d2af4449ffc439c2ef7d744a3e7b12",
	"capabilities/com.docker.sandbox/git-identity@1.schema.json":   "df8997e0c26722c7a8fd29c2dbb5124e26cd9080c686e7039eb6b2d667200d6d",
	"capabilities/com.docker.sandbox/host-mount@1.schema.json":     "5a27195971315bbf25e3ce472288c237c7cb29d780b48040e30c5990988ca85f",
	"capabilities/com.docker.sandbox/kit-registry@1.schema.json":   "416244181629a7a59e404d02e7e913962bc12ba17c8042d127fb6671d3c2e377",
	"capabilities/com.docker.sandbox/lifecycle@1.schema.json":      "77157604b65c6e9162d9d649405a863cbaa715586f470f273edce09a3581cc34",
	"capabilities/com.docker.sandbox/long-running@1.schema.json":   "9579d79f36fa7014115d4d4134e9dd88b53534e54c847fa87139aa0ac2e9e6b7",
	"capabilities/com.docker.sandbox/network-policy@1.schema.json": "f22f59e7885036367870eca6ff248afa02e90023fac12da158dc9ebf37bf83c5",
	"capabilities/com.docker.sandbox/network-policy@2.schema.json": "a3adef40b98c02fcbea6cf80d417dcffff00e28fdfb5dda90b65cac1eaa200a6",
	"capabilities/com.docker.sandbox/port@1.schema.json":           "8660814414c49b94aa6c5e51c74d88c987945131e272794767557f9f9ee9e7f5",
	"capabilities/com.docker.sandbox/privileged@1.schema.json":     "21fdd9ee9e45f77d4f1bb390b31563aa904c1f2865e62d99ccfa25adf74f4fe8",
	"capabilities/com.docker.sandbox/resources@1.schema.json":      "c9abc295a6363c7c55f5d760e2d44065939aa3a7f5e8fa0789b6fb4133c821b3",
	"capabilities/com.docker.sandbox/sbx@1.schema.json":            "569a591f4645cebaedc7db540c19431dafdfb07f9a8b07e8be925d74ffa5421d",
	"capabilities/com.docker.sandbox/ssh-agent@1.schema.json":      "a28cc35c732a1ca5a34e040cf3968b5439ac94333738c6c893fefba1eabbce9f",
	"capabilities/com.docker.sandbox/usb-device@1.schema.json":     "84d9fd18502a23f2d23ec6031ada5588ea1e8b8ad37a490197cd26662771ce2f",
	"capabilities/com.docker.sandbox/volume@1.schema.json":         "8ff17346cc5037fd300479d3ad095bc5511085616fa2eb5efd1c4f285e4f007f",
	"definitions/kit-arg.schema.json":                              "2817f5948805d6a431b41f64155b19848bb3c903efab2908648586413480eab3",
	"kit.schema.json":                                              "ce7cf3cb659c5e0b3938d9791044049da858b72a0eb3a34da4316d7cbc8719ac",
}
