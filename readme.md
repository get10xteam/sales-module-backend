create sales module backend di github
pemahaman multi tanent / single ke mas gilang
structure beda 
masalah login, aplikasi terpisah

usecase 
DRAFT -> PUBLISH
contoh aturan workflow approval sangat dinamis
penempatan flow di coding, bisa jadi berubah 
bagaimana configurasi di DB, sudah lebih mudah

# level table
- id int primary key => 100,200,300 => untuk ordering hierarcy 
- name text not null
- rules JSONB not null => defines ability (misal viewer), ini di pikirin nanti

# client table
- id serial primary key
- name text not null
- logo_url text null

# user table
- id serial primary key
- name text not null
- email text not null unique
- emailconfirmed boolean not null default false
- password text
- profileimg_url text
- createts timestamptz not null default now()
- level_id int references level(id)
- parent_id references user(id)

# status table
- code text primary key
- name text not null
- description text null

# create table opportunities with columns
- owner_id references user(id)
- assignee_id references user(id)
- client_id references client(id)
- status_code references status(code)
- name
- description
- talent_budget
- non_talent_budget
- revenue
- expected_profitability_percentage (generated column) 
generated always as (
    revenue - talent_budget - non_talent_budget
 ) // tambah coalesce
- expected_profitability_amount (generated column)

# designing approval opportunity logic
- [X] ability to accept multiple hierarchy levels (parent_id) 
- [ ] ability to control access of each user 
=> ambil view based on user- hierarchy

# table action (soon)
pakai rule
based on status
ketika draft -> bisa action apa saja yang bisa
misal
draft -> release if revenue lebih dari 1M

