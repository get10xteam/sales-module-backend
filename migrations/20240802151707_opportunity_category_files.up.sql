BEGIN
;

create table opportunity_category_files(
    id serial primary key,
    opportunity_category_id int references opportunity_categories(id),
    url text not null,
    name text null,
    version int not null,
    creator_id int references users(id),
    create_ts timestamptz not null default now()
);

COMMIT;