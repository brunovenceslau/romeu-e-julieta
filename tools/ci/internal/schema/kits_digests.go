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
	"capabilities/com.docker.sandbox/acp@1.schema.json":                        "61e9ddfd683c40006232bbb9a00b670d52d400603b4e9bb029564a905e178e8f",
	"capabilities/com.docker.sandbox/agent-context@1.schema.json":              "222f60d943ddee7e748826db653ce3b5fa978147e48048c11ef38144fb8f6b41",
	"capabilities/com.docker.sandbox/agent-context@2.schema.json":              "a5453d145fbc9c81f1e4b9a03203ebbe5335eb066f486b2b47a3da42b3bac246",
	"capabilities/com.docker.sandbox/agent-interactive-sessions@1.schema.json": "d5640c363386977ce6c82311969d3713fd8e4e92afbd70b7231c2339e3ea962e",
	"capabilities/com.docker.sandbox/agent-sessions@1.schema.json":             "8223514d36f15bd200a9938a15f020148054eb31a8acc05c1a208c509d15ecac",
	"capabilities/com.docker.sandbox/agent-skill@1.schema.json":                "91dd731450f0d0f1a3494106fc336f478e606b0ff0201bc669aa1d771c662298",
	"capabilities/com.docker.sandbox/agent-skills@1.schema.json":               "fb3021c7ab6efc0d1c06915d5e1257ec4fb6791434acac84586c8bebf1477179",
	"capabilities/com.docker.sandbox/credential@1.schema.json":                 "8fcb59a22ee96e3d77fff6da1dc409b530a7e917cfeacd0e86918a8876f2e217",
	"capabilities/com.docker.sandbox/git-identity@1.schema.json":               "df8997e0c26722c7a8fd29c2dbb5124e26cd9080c686e7039eb6b2d667200d6d",
	"capabilities/com.docker.sandbox/host-mount@1.schema.json":                 "130b62338ac3d4ae947fa808094304330090f853422f10f95e245a2ac1417657",
	"capabilities/com.docker.sandbox/kit-registry@1.schema.json":               "416244181629a7a59e404d02e7e913962bc12ba17c8042d127fb6671d3c2e377",
	"capabilities/com.docker.sandbox/lifecycle@1.schema.json":                  "77157604b65c6e9162d9d649405a863cbaa715586f470f273edce09a3581cc34",
	"capabilities/com.docker.sandbox/long-running@1.schema.json":               "9579d79f36fa7014115d4d4134e9dd88b53534e54c847fa87139aa0ac2e9e6b7",
	"capabilities/com.docker.sandbox/network-policy@1.schema.json":             "dc08ce9e6506dcf89744f8d003ebb727cc26b56749c37390988c028872272a39",
	"capabilities/com.docker.sandbox/network-policy@2.schema.json":             "8497a24e6378cafb28f0e2f830a3681dc93a878e38386c3ad23e1af988cbd98b",
	"capabilities/com.docker.sandbox/port@1.schema.json":                       "4a43bc70d187f675f8fad8119a229ae2e197bb0378c317bb7f01c2e071959ff7",
	"capabilities/com.docker.sandbox/privileged@1.schema.json":                 "21fdd9ee9e45f77d4f1bb390b31563aa904c1f2865e62d99ccfa25adf74f4fe8",
	"capabilities/com.docker.sandbox/resources@1.schema.json":                  "a123af6a3bb0c53cd4f0a10b1d37df2f823ec06b5f18a2c70c25b1ef9cc2a0b6",
	"capabilities/com.docker.sandbox/sbx@1.schema.json":                        "569a591f4645cebaedc7db540c19431dafdfb07f9a8b07e8be925d74ffa5421d",
	"capabilities/com.docker.sandbox/ssh-agent@1.schema.json":                  "a28cc35c732a1ca5a34e040cf3968b5439ac94333738c6c893fefba1eabbce9f",
	"capabilities/com.docker.sandbox/usb-device@1.schema.json":                 "b4398c03cac4e9de33d6deb711c8c1b3c8bc35a8325aaf3af1fafe0a984eadf7",
	"capabilities/com.docker.sandbox/volume@1.schema.json":                     "027eeb47a2078c85f2411cc86d1dfc7c405d8653326ee41acd35db0f7c542a16",
	"definitions/kit-arg.schema.json":                                          "2817f5948805d6a431b41f64155b19848bb3c903efab2908648586413480eab3",
	"kit.schema.json":                                                          "ae3eac2e6798165f474fe2d62c672e7c1e550e815735b43973fe7444d59b1605",
}
