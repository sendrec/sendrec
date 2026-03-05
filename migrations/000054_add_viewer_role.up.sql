ALTER TABLE organization_members DROP CONSTRAINT organization_members_role_check;
ALTER TABLE organization_members ADD CONSTRAINT organization_members_role_check
  CHECK (role IN ('owner', 'admin', 'member', 'viewer'));

ALTER TABLE organization_invites DROP CONSTRAINT organization_invites_role_check;
ALTER TABLE organization_invites ADD CONSTRAINT organization_invites_role_check
  CHECK (role IN ('admin', 'member', 'viewer'));
