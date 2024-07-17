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
    -- TODO confirm data type for talent_budget, non_talent_budget, revenue
    talent_budget DOUBLE PRECISION,
    non_talent_budget DOUBLE PRECISION,
    revenue DOUBLE PRECISION,
    expected_profitability_percentage DOUBLE PRECISION GENERATED ALWAYS AS (
        (
            revenue - talent_budget - non_talent_budget
        ) / revenue
    ) STORED,
    expected_profitability_amount DOUBLE PRECISION GENERATED ALWAYS AS (
        (
            revenue - talent_budget - non_talent_budget
        )
    ) STORED,
    create_ts timestamptz not null default now()
);

COMMIT;