BEGIN;
CREATE TABLE IF NOT EXISTS public.tavern_credits (
    user_id bigint PRIMARY KEY,
    quota bigint NOT NULL,
    granted_quota bigint NOT NULL,
    created_time bigint
);
ALTER TABLE public.tavern_credits OWNER TO :"tavern_owner";
SET LOCAL ROLE :"tavern_owner";
SELECT user_id, quota, granted_quota, created_time FROM public.tavern_credits LIMIT 0;
COMMIT;
