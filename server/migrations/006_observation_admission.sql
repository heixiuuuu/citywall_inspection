ALTER TABLE observations
    ADD COLUMN admitted_at DATETIME(3) NULL,
    ADD COLUMN admitted_by BIGINT UNSIGNED NULL,
    ADD COLUMN no_issue_reason TEXT NULL;

UPDATE observations o
JOIN (
    SELECT observation_id, linked_at, linked_by,
           ROW_NUMBER() OVER (PARTITION BY observation_id ORDER BY linked_at, issue_id) AS sequence_no
    FROM issue_observations
) linked ON linked.observation_id = o.id AND linked.sequence_no = 1
SET o.admitted_at = linked.linked_at, o.admitted_by = linked.linked_by
WHERE o.status = 'ACCEPTED';

UPDATE observations o
JOIN audit_events admission_event ON admission_event.id = (
    SELECT MAX(previous.id) FROM audit_events previous
    WHERE previous.entity_type = 'observation' AND previous.entity_id = o.id
      AND previous.action = 'admit' AND TRIM(COALESCE(previous.reason, '')) <> ''
)
SET o.admitted_at = admission_event.created_at, o.admitted_by = admission_event.actor_id, o.no_issue_reason = admission_event.reason
WHERE o.status = 'ACCEPTED' AND o.admitted_at IS NULL;
