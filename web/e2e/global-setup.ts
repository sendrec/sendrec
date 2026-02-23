import { truncateAllTables } from "./helpers/db";
import { createVerifiedUser } from "./helpers/auth";

export default async function globalSetup() {
  await truncateAllTables();
  await createVerifiedUser();
}
