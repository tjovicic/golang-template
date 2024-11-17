CREATE TABLE IF NOT EXISTS sensor_data
(
    id          uuid        default gen_random_uuid(),
    timestamp   TIMESTAMP WITH TIME ZONE              NOT NULL,
    temperature NUMERIC,
    humidity    NUMERIC,
    pressure    NUMERIC,
    latitude    NUMERIC,
    longitude   NUMERIC,
    created     timestamptz default current_timestamp NOT NULL,
    updated     timestamptz default current_timestamp NOT NULL,
    PRIMARY KEY (id)
);
