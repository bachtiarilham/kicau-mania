-- Group roles are retained because other databases can use them.
REVOKE ALL ON ALL TABLES IN SCHEMA cloud_read FROM kicau_cloud_reader, kicau_cloud_admin_reader, kicau_cloud_recap_maintainer;
REVOKE USAGE ON SCHEMA cloud_read FROM kicau_cloud_reader, kicau_cloud_admin_reader, kicau_cloud_recap_maintainer;
