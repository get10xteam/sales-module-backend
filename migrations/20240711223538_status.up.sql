BEGIN
;

create table statuses(
    code text primary key,
    name text,
    description text,
    create_ts timestamptz not null default now()
);

COMMIT;