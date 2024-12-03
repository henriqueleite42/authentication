CREATE SCHEMA authentication;

CREATE TYPE "SignInProviderEnum" AS ENUM ('FACEBOOK', 'GOOGLE', 'DISCORD');

CREATE TABLE authentication.accounts (
	id uuid NOT NULL,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY(id)
);

CREATE TABLE authentication.refresh_tokens (
	account_id uuid NOT NULL,
	refresh_token varchar(64) NOT NULL UNIQUE,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY(account_id, refresh_token),
	CONSTRAINT rt_refresh_token_idx UNIQUE (refresh_token)
);

CREATE TABLE authentication.magic_link_codes (
	account_id uuid NOT NULL,
	code varchar(16) NOT NULL,
	is_first_access boolean NOT NULL,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY(account_id)
);

CREATE TABLE authentication.sign_in_providers (
	account_id uuid NOT NULL,
	provider "SignInProviderEnum" NOT NULL,
	provider_id varchar NOT NULL,
	access_token varchar NOT NULL,
	refresh_token varchar,
	access_token varchar NOT NULL,
	expires_at timestamp NOT NULL,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY(account_id, provider, provider_id)
);

CREATE TABLE authentication.email_addresses (
	account_id uuid NOT NULL,
	email_address varchar NOT NULL,
	verified_at timestamp,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY(account_id, email_address),
	CONSTRAINT ea_email_address_idx UNIQUE (email_address)
);

CREATE TABLE authentication.phone_numbers (
	account_id uuid NOT NULL,
	country_code varchar NOT NULL,
	phone_number varchar NOT NULL,
	verified_at timestamp,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY(account_id, country_code, phone_number),
	CONSTRAINT pn_country_code_phone_number_idx UNIQUE (country_code, phone_number)
);

ALTER TABLE authentication.refresh_tokens ADD CONSTRAINT refresh_tokens_account_id_fkey FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;

ALTER TABLE authentication.magic_link_codes ADD CONSTRAINT magic_link_codes_account_id_fkey FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;

ALTER TABLE authentication.sign_in_providers ADD CONSTRAINT sign_in_providers_account_id_fkey FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;

ALTER TABLE authentication.email_addresses ADD CONSTRAINT email_addresses_account_id_fkey FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;

ALTER TABLE authentication.phone_numbers ADD CONSTRAINT phone_numbers_account_id_fkey FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;
