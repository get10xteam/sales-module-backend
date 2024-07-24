INSERT INTO
    levels("name")
VALUES
    ('Director'),
    ('Director of Operation'),
    ('Manager'),
    ('Sales');

INSERT INTO
    clients (id, "name", logo_url)
VALUES
    (
        1,
        'client test',
        'https://loremflickr.com/cache/resized/65535_53801147457_2a59fe2e69_300_300_nofilter.jpg'
    );

INSERT INTO
    statuses (code, description)
VALUES
    ('DRAFT', 'drafting mode');