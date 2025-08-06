TRUNCATE TABLE auth.users RESTART IDENTITY CASCADE;
TRUNCATE TABLE booking.bookings RESTART IDENTITY CASCADE;
TRUNCATE TABLE event.events RESTART IDENTITY CASCADE;
TRUNCATE TABLE event.seats RESTART IDENTITY CASCADE;

INSERT INTO auth.users (email, password_hash) VALUES 
('user@example.com', 'aaa'),
('admin@example.com', 'abc');

INSERT INTO event.events (title, description) VALUES
('Shrek', 'A mean lord exiles fairytale creatures to the swamp of a grumpy ogre, who must go on a quest and rescue a princess for the lord in order to get his land back.'),
('Agil', 'An angelically patient person welcomes a group of dysfunctional friends into their life, who then embark on a quest to test every last one of his boundaries for their own amusement and personal gain.');

INSERT INTO event.seats (event_id, seat_number, row_number, sector, status) 
SELECT
	1 AS event_id,
	'A' || s.i AS seat_number,
	1 AS row_number,
	'A' AS sector,
	'AVAILABLE' AS status
FROM generate_series(1, 10) AS s(i);

INSERT INTO event.seats (event_id, seat_number, row_number, sector, status) 
SELECT
	2 AS event_id,
	'A' || s.i AS seat_number,
	1 AS row_number,
	'Main' AS sector,
	'AVAILABLE' AS status
FROM generate_series(1, 10) AS s(i);

SELECT setval('event.events_id_seq', (SELECT MAX(id) FROM event.events));