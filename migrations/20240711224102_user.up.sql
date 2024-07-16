BEGIN
;

create table users (
    id serial primary key,
    name text not null,
    email text not null unique,
    email_confirmed boolean not null default false,
    password text,
    profile_img_url text,
    create_ts timestamptz not null default now(),
    level_id int references levels(id),
    parent_id int references users(id)
);

create table user_email_verifications (
    id uuid primary key default gen_random_uuid(),
    create_ts timestamptz not null default now(),
    expiry_ts timestamptz not null default now() + '1 hour' :: interval,
    used_ts timestamptz,
    user_id int references users(id),
    purpose int not null,
    meta jsonb
);

create table user_sessions (
    id uuid primary key default gen_random_uuid(),
    user_id int not null references users(id),
    ip_addr inet,
    create_ts timestamptz not null default now(),
    expiry_ts timestamptz default now() + '30 minutes' :: interval,
    logout_ts timestamptz,
    user_agent text,
    user_agent_description text
);

COMMIT;