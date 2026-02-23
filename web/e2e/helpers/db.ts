import pg from "pg";

const DATABASE_URL =
  process.env.DATABASE_URL ||
  "postgres://sendrec:sendrec@localhost:5433/sendrec";

export async function query(sql: string, params?: unknown[]): Promise<void> {
  const client = new pg.Client({ connectionString: DATABASE_URL });
  await client.connect();
  try {
    await client.query(sql, params);
  } finally {
    await client.end();
  }
}

export async function queryRows<T>(
  sql: string,
  params?: unknown[]
): Promise<T[]> {
  const client = new pg.Client({ connectionString: DATABASE_URL });
  await client.connect();
  try {
    const result = await client.query(sql, params);
    return result.rows as T[];
  } finally {
    await client.end();
  }
}

export async function truncateAllTables(): Promise<void> {
  await query(`
    TRUNCATE users, videos, refresh_tokens, password_resets,
             email_confirmations, video_comments, video_views,
             folders, tags, video_tags, notification_preferences,
             api_keys, webhook_deliveries, user_branding,
             cta_clicks, view_milestones, video_viewers
    CASCADE
  `);
}
