-- Grant only the built-in Agent public weather access; custom policies stay unchanged.
UPDATE agent_definition_versions
SET permissions_json = JSON_ARRAY_APPEND(permissions_json, '$', 'weather.read')
WHERE definition_uuid = CONCAT('embedded:', agent_uuid)
  AND NOT JSON_CONTAINS(permissions_json, JSON_QUOTE('weather.read'));

UPDATE agent_definition_versions
SET scopes_json = JSON_ARRAY_APPEND(scopes_json, '$',
    JSON_OBJECT('resource_type', 'weather', 'resource_id', '*', 'actions', JSON_ARRAY('read')))
WHERE definition_uuid = CONCAT('embedded:', agent_uuid)
  AND NOT JSON_CONTAINS(scopes_json,
    JSON_OBJECT('resource_type', 'weather', 'resource_id', '*', 'actions', JSON_ARRAY('read')));
