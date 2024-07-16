BEGIN
;

create table clients(
    id serial primary key,
    name text,
    logo_url text,
    create_ts timestamptz not null default now()
);

COMMIT;