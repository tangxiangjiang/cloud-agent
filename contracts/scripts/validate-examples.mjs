import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, "..");
const schemasDir = join(root, "schemas");
const examplesDir = join(root, "examples");

const ajv = new Ajv2020({ allErrors: true, strict: false });
addFormats(ajv);

for (const name of readdirSync(schemasDir)) {
  if (!name.endsWith(".schema.json")) continue;
  const raw = JSON.parse(readFileSync(join(schemasDir, name), "utf8"));
  ajv.addSchema(raw);
}

const cases = [
  ["task.schema.json", "task.json"],
  ["task-event.schema.json", "task-event-status.json"],
  ["task-event.schema.json", "task-event-assistant-delta.json"],
  ["task-event.schema.json", "task-event-done.json"],
  ["workflow-bundle.schema.json", "workflow-bundle.json"],
  ["workflow-run.schema.json", "workflow-run.json"],
  ["review-request.schema.json", "review-approve.json"],
  ["review-request.schema.json", "review-reject.json"],
  ["revise-request.schema.json", "revise-request.json"],
  ["node-diff.schema.json", "node-diff.json"],
  ["task-unit.schema.json", "task-unit.json"],
];

let failed = 0;
for (const [schemaFile, exampleFile] of cases) {
  const schema = JSON.parse(readFileSync(join(schemasDir, schemaFile), "utf8"));
  const data = JSON.parse(readFileSync(join(examplesDir, exampleFile), "utf8"));
  const validate = ajv.getSchema(schema.$id) ?? ajv.compile(schema);
  const ok = validate(data);
  if (!ok) {
    failed += 1;
    console.error(`FAIL ${exampleFile} vs ${schemaFile}`);
    console.error(validate.errors);
  } else {
    console.log(`OK   ${exampleFile}`);
  }
}

if (failed) {
  console.error(`\n${failed} case(s) failed`);
  process.exit(1);
}
console.log("\nAll examples valid.");
