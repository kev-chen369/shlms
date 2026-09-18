ALTER TABLE channel_capabilities DROP CONSTRAINT channel_capabilities_evidence_scope_fk;
DROP TABLE channel_capability_evidence;
DROP TABLE channel_capabilities;
DROP FUNCTION prevent_channel_capability_evidence_mutation();
DROP FUNCTION prevent_channel_capability_key_mutation();
DROP FUNCTION channel_capability_text_valid(TEXT, INTEGER, BOOLEAN);
