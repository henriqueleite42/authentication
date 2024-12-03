ALTER TABLE authentication.phone_numbers DROP CONSTRAINT phone_numbers_account_id_fkey;

ALTER TABLE authentication.email_addresses DROP CONSTRAINT email_addresses_account_id_fkey;

ALTER TABLE authentication.sign_in_providers DROP CONSTRAINT sign_in_providers_account_id_fkey;

ALTER TABLE authentication.magic_link_codes DROP CONSTRAINT magic_link_codes_account_id_fkey;

ALTER TABLE authentication.refresh_tokens DROP CONSTRAINT refresh_tokens_account_id_fkey;

DROP TABLE authentication.phone_numbers;

DROP TABLE authentication.email_addresses;

DROP TABLE authentication.sign_in_providers;

DROP TABLE authentication.magic_link_codes;

DROP TABLE authentication.refresh_tokens;

DROP TABLE authentication.accounts;

DROP TYPE "SignInProviderEnum";

DROP SCHEMA authentication;
