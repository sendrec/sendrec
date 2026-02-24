import { truncateAllTables, closePool } from "./helpers/db";

export default async function globalTeardown() {
  await truncateAllTables();
  await closePool();
}
