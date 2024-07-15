BEGIN
;

create table opportunities(
    id serial primary key,
    owner_id int references users(id),
    assignee_id int references users(id),
    client_id int references clients(id),
    status_code text references statuses(code),
    name text not null,
    description text,
    talent_budget numeric(7, 5),
    non_talent_budget numeric(7, 5),
    revenue numeric(7, 5),
    expected_profitability_percentage DOUBLE PRECISION GENERATED ALWAYS AS (
        (
            revenue - talent_budget - non_talent_budget
        ) / revenue
    ) STORED,
    expected_profitability_amount numeric(7, 5) GENERATED ALWAYS AS (
        (
            revenue - talent_budget - non_talent_budget
        )
    ) STORED,
    create_ts timestamptz not null default now()
);

COMMIT;