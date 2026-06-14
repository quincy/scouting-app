-- +goose Up

-- Rename existing roles to match Scoutbook position names
UPDATE roles SET name = 'Scoutmaster' WHERE name = 'scoutmaster';
UPDATE roles SET name = 'Assistant Scoutmaster' WHERE name = 'asst_scoutmaster';
UPDATE roles SET name = 'Scouts BSA' WHERE name = 'scout';

-- Insert remaining Scoutbook position roles
INSERT INTO roles (name) VALUES
    ('Assistant Patrol Leader'),
    ('Assistant Senior Patrol Leader'),
    ('Chaplain Aide'),
    ('Chartered Organization Rep.'),
    ('Committee Chairman'),
    ('Committee Member'),
    ('Den Chief'),
    ('Executive Officer'),
    ('Historian'),
    ('Librarian'),
    ('Life-to-Eagle Coordinator'),
    ('OA Unit Representative'),
    ('Outdoor Ethics Guide'),
    ('Patrol Admin'),
    ('Patrol Leader'),
    ('Quartermaster'),
    ('Scribe'),
    ('Senior Patrol Leader'),
    ('Troop Admin'),
    ('Troop Guide'),
    ('Unit Advancement Chair'),
    ('Unit College Scouter Reserve'),
    ('Unit Outdoors / Activities Chair'),
    ('Unit Public Relations Chair'),
    ('Unit Scouter Reserve'),
    ('Unit Training Chair'),
    ('Unit Treasurer'),
    ('Webmaster'),
    ('Youth Protection Champion')
ON CONFLICT (name) DO NOTHING;

-- +goose Down

UPDATE roles SET name = 'scoutmaster' WHERE name = 'Scoutmaster';
UPDATE roles SET name = 'asst_scoutmaster' WHERE name = 'Assistant Scoutmaster';
UPDATE roles SET name = 'scout' WHERE name = 'Scouts BSA';

DELETE FROM roles WHERE name IN (
    'Assistant Patrol Leader',
    'Assistant Senior Patrol Leader',
    'Chaplain Aide',
    'Chartered Organization Rep.',
    'Committee Chairman',
    'Committee Member',
    'Den Chief',
    'Executive Officer',
    'Historian',
    'Librarian',
    'Life-to-Eagle Coordinator',
    'OA Unit Representative',
    'Outdoor Ethics Guide',
    'Patrol Admin',
    'Patrol Leader',
    'Quartermaster',
    'Scribe',
    'Senior Patrol Leader',
    'Troop Admin',
    'Troop Guide',
    'Unit Advancement Chair',
    'Unit College Scouter Reserve',
    'Unit Outdoors / Activities Chair',
    'Unit Public Relations Chair',
    'Unit Scouter Reserve',
    'Unit Training Chair',
    'Unit Treasurer',
    'Webmaster',
    'Youth Protection Champion'
);
