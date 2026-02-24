import pg from "pg";

const DATABASE_URL =
  process.env.DATABASE_URL ||
  "postgres://sendrec:sendrec@localhost:5433/sendrec";

const pool = new pg.Pool({ connectionString: DATABASE_URL, max: 3 });

export async function query(sql: string, params?: unknown[]): Promise<void> {
  await pool.query(sql, params);
}

export async function queryRows<T>(
  sql: string,
  params?: unknown[]
): Promise<T[]> {
  const result = await pool.query(sql, params);
  return result.rows as T[];
}

export async function truncateAllTables(): Promise<void> {
  await query(`
    TRUNCATE users, videos, refresh_tokens, password_resets,
             email_confirmations, video_comments, video_views,
             folders, tags, video_tags, notification_preferences,
             api_keys, webhook_deliveries, user_branding,
             cta_clicks, view_milestones, video_viewers,
             segment_engagement
    CASCADE
  `);
}

export async function closePool(): Promise<void> {
  await pool.end();
}
