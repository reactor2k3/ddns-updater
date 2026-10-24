// The editable frontend is embedded directly from internal/server/web.
// Regenerate the server's provider validation schema after editing providers.js.
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const root = path.resolve(__dirname, "..");
const source = path.join(root, "internal/server/web/providers.js");
const providers = vm.runInNewContext(
  fs.readFileSync(source, "utf8") + "\nPROVIDERS",
);
if (
  !Array.isArray(providers) ||
  providers.length === 0 ||
  new Set(providers.map((p) => p.id)).size !== providers.length
) {
  throw new Error("Invalid provider catalog");
}
fs.writeFileSync(
  path.join(root, "internal/control/catalog.json"),
  JSON.stringify(providers, null, 2) + "\n",
);
console.log(`Validated ${providers.length} provider schemas.`);
