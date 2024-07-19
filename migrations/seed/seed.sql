INSERT INTO
    levels("name")
VALUES
    ('Director');

INSERT INTO
    levels("name")
VALUES
    ('Director of Operation');

INSERT INTO
    levels("name")
VALUES
    ('Manager');

INSERT INTO
    levels("name")
VALUES
    ('Sales');

INSERT INTO
    clients (id, "name", logo_url)
VALUES
    (1, 'client test', NULL);

INSERT INTO
    statuses (code, "name", description)
VALUES
    ('DRAFT', 'draft', 'drafting mode', now());