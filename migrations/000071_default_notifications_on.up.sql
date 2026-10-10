-- New accounts start with view and comment emails on: the "they watched it"
-- email is the moment that brings senders back. A trigger covers every path
-- that creates a user (signup, SSO, SCIM). Existing accounts are untouched;
-- with no row they stay off, as before.
CREATE FUNCTION default_notification_preferences() RETURNS trigger AS $$
BEGIN
    INSERT INTO notification_preferences (user_id, view_notification)
    VALUES (NEW.id, 'views_and_comments')
    ON CONFLICT (user_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_default_notification_preferences
    AFTER INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION default_notification_preferences();
