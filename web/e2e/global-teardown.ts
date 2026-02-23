import { truncateAllTables } from "./helpers/db";

export default async function globalTeardown() {
  await truncateAllTables();
}
