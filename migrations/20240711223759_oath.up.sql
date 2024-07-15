BEGIN
;

create table oauth_authorizations(
    provider int,
    state uuid,
    subject text,
    email text,
    name text,
    destination_url text,
    expiry timestamptz not null default now() + '4 minutes' :: interval,
    create_ts timestamptz not null default now(),
    exchange_ts timestamptz,
    primary key(provider, state)
);

COMMIT;