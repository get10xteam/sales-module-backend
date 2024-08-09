BEGIN
;

create table opportunity_categories(
    id serial primary key,
    opportunity_id int references opportunities(id),
    name text not null,
    create_ts timestamptz not null default now()
);

COMMIT;