import { truncateAllTables } from "./helpers/db";
import { createVerifiedUser, createSecondUser } from "./helpers/auth";

export default async function globalSetup() {
  await truncateAllTables();
  await createVerifiedUser();
  await createSecondUser();
}
