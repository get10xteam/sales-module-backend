BEGIN
;

create sequence if not exists level_sequence increment 100 start 100;

create table levels(
    id bigint primary key default nextval('level_sequence'),
    create_ts timestamptz not null default now()
);

COMMIT;