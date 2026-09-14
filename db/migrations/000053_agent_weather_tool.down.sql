UPDATE agent_definition_versions
SET permissions_json = JSON_REMOVE(permissions_json,
    JSON_UNQUOTE(JSON_SEARCH(permissions_json, 'one', 'weather.read')))
WHERE definition_uuid = CONCAT('embedded:', agent_uuid)
  AND JSON_SEARCH(permissions_json, 'one', 'weather.read') IS NOT NULL;

UPDATE agent_definition_versions
SET scopes_json = JSON_REMOVE(scopes_json,
    REPLACE(JSON_UNQUOTE(JSON_SEARCH(scopes_json, 'one', 'weather', NULL, '$[*].resource_type')), '.resource_type', ''))
WHERE definition_uuid = CONCAT('embedded:', agent_uuid)
  AND JSON_SEARCH(scopes_json, 'one', 'weather', NULL, '$[*].resource_type') IS NOT NULL;
