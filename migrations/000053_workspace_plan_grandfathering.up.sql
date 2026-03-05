ALTER TABLE organizations ADD COLUMN plan_inherited_from UUID REFERENCES users(id);

UPDATE organizations o
SET subscription_plan = u.subscription_plan,
    plan_inherited_from = u.id,
    updated_at = now()
FROM organization_members om
JOIN users u ON u.id = om.user_id
WHERE om.organization_id = o.id
  AND om.role = 'owner'
  AND o.subscription_plan = 'free'
  AND u.subscription_plan IN ('pro', 'business');
