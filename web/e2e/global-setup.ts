import { truncateAllTables, query } from "./helpers/db";
import { createVerifiedUser, createSecondUser, TEST_USER } from "./helpers/auth";

export default async function globalSetup() {
  await truncateAllTables();
  await createVerifiedUser();
  await createSecondUser();
  await query(
    "UPDATE users SET subscription_plan = 'pro' WHERE email = $1",
    [TEST_USER.email]
  );
}
