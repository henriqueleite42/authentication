package services

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/econominhas/authentication/internal/adapters"
	"github.com/econominhas/authentication/internal/models"
)

type AccountService struct {
	GoogleAdapter   adapters.SignInProviderAdapter
	FacebookAdapter adapters.SignInProviderAdapter
	TokenAdapter    adapters.TokenAdapter
	EmailAdapter    adapters.EmailAdapter
	SmsAdapter      adapters.SmsAdapter

	Db *sql.DB

	AccountRepository       models.AccountRepository
	RefreshTokenRepository  models.RefreshTokenRepository
	MagicLinkCodeRepository models.MagicLinkCodeRepository
}

type genAuthOutputInput struct {
	db *sql.Tx

	accountId     string
	isFirstAccess bool
	refresh       bool
}

type createFromExternalProviderInput struct {
	db *sql.Tx

	providerService *adapters.SignInProviderAdapter
	providerType    string
	code            string
	originUrl       string
}

func (serv *AccountService) genAuthOutput(i *genAuthOutputInput) (*models.AuthOutput, error) {
	var wg sync.WaitGroup
	var refreshToken *models.CreateRefreshTokenOutput
	var accessToken *adapters.GenAccessOutput
	var err error

	if i.refresh {
		wg.Add(1)
		defer wg.Done()
		go func() {
			refreshToken, err = serv.RefreshTokenRepository.Create(&models.CreateRefreshTokenInput{
				Db: i.db,

				AccountId: i.accountId,
			})
		}()
	}

	wg.Add(1)
	defer wg.Done()
	go func() {
		accessToken, err = serv.TokenAdapter.GenAccess(&adapters.GenAccessInput{
			AccountId: i.accountId,
		})
	}()

	wg.Wait()

	if err != nil {
		i.db.Rollback()
		return nil, errors.New("fail to generate auth output")
	}

	i.db.Commit()

	return &models.AuthOutput{
		AccessToken:  accessToken.AccessToken,
		ExpiresAt:    accessToken.ExpiresAt,
		RefreshToken: refreshToken.RefreshToken,
	}, nil
}

func (serv *AccountService) createFromExternal(i *createFromExternalProviderInput) (*models.AuthOutput, error) {
	exchangeCode, err := (*i.providerService).ExchangeCode(&adapters.ExchangeCodeInput{
		Code:      i.code,
		OriginUrl: i.originUrl,
	})
	if err != nil {
		i.db.Rollback()
		return nil, errors.New("fail to exchange code")
	}

	hasRequiredScopes := (*i.providerService).HasRequiredScopes(exchangeCode.Scopes)
	if !hasRequiredScopes {
		i.db.Rollback()
		return nil, errors.New("missing scopes")
	}

	providerData, err := (*i.providerService).GetUserData(exchangeCode.AccessToken)
	if err != nil {
		i.db.Rollback()
		return nil, errors.New("fail to get external user data")
	}

	if !providerData.IsEmailVerified {
		i.db.Rollback()
		return nil, errors.New("unverified email")
	}

	relatedAccounts, err := serv.AccountRepository.GetManyByProvider(&models.GetManyAccountsByProviderInput{
		Db: i.db,

		ProviderId:   providerData.Id,
		ProviderType: i.providerType,
		Email:        providerData.Email,
	})
	if err != nil {
		i.db.Rollback()
		return nil, errors.New("fail to get related accounts")
	}

	var accountId string
	var isFirstAccess bool

	if len(relatedAccounts) > 0 {
		sameEmail := new(models.GetManyAccountsByProviderOutput)
		sameProvider := new(models.GetManyAccountsByProviderOutput)
		for _, v := range relatedAccounts {
			if v.Email == providerData.Email {
				sameEmail = &v
			}
			if v.ProviderId == providerData.Id && v.ProviderType == i.providerType {
				sameProvider = &v
			}
			if sameEmail != nil && sameProvider != nil {
				break
			}
		}

		/*
		 * Has an account with the same email, and it
		 * isn't linked with another provider with the same type
		 */
		if sameEmail != nil && sameProvider == nil && sameEmail.ProviderType != i.providerType {
			accountId = sameEmail.AccountId
		}

		/*
		 * Account with same provider id (it can have a different email,
		 * in case that the user updated it in provider or on our platform)
		 * More descriptive IF:
		 * if ((sameProviderId && !sameEmail) || (sameProviderId && sameEmail)) {
		 */
		if sameProvider != nil {
			accountId = sameProvider.AccountId
		}

		if accountId == "" {
			i.db.Rollback()
			return nil, errors.New("fail to relate account")
		}
	} else {
		result, err := serv.AccountRepository.Create(&models.CreateAccountInput{
			Db: i.db,

			Email: providerData.Email,
			SignInProviders: []models.CreateAccountSignInProvider{
				{
					Id:           providerData.Id,
					Type:         i.providerType,
					AccessToken:  exchangeCode.AccessToken,
					RefreshToken: &exchangeCode.RefreshToken,
					ExpiresAt:    exchangeCode.ExpiresAt,
				},
			},
		})
		if err != nil {
			i.db.Rollback()
			return nil, errors.New("fail to create account")
		}

		accountId = result.Id
		isFirstAccess = true
	}

	return serv.genAuthOutput(&genAuthOutputInput{
		accountId:     accountId,
		isFirstAccess: isFirstAccess,
		refresh:       true,
	})
}

func (serv *AccountService) CreateFromGoogleProvider(i *models.CreateAccountFromExternalProviderInput) (*models.AuthOutput, error) {
	tx, err := serv.Db.Begin()
	if err != nil {
		return nil, errors.New("fail to create transaction")
	}

	return serv.createFromExternal(&createFromExternalProviderInput{
		db: tx,

		providerService: &serv.GoogleAdapter,
		providerType:    "GOOGLE",
		code:            i.Code,
		originUrl:       i.OriginUrl,
	})
}

func (serv *AccountService) CreateFromFacebookProvider(i *models.CreateAccountFromExternalProviderInput) (*models.AuthOutput, error) {
	tx, err := serv.Db.Begin()
	if err != nil {
		return nil, errors.New("fail to create transaction")
	}

	return serv.createFromExternal(&createFromExternalProviderInput{
		db: tx,

		providerService: &serv.FacebookAdapter,
		providerType:    "FACEBOOK",
		code:            i.Code,
		originUrl:       i.OriginUrl,
	})
}

func (serv *AccountService) CreateFromEmailProvider(i *models.CreateAccountFromEmailInput) error {
	tx, err := serv.Db.Begin()
	if err != nil {
		return errors.New("fail to create transaction")
	}

	var accountId string
	var isFirstAccess bool

	existentAccount, err := serv.AccountRepository.GetByEmail(&models.GetAccountByEmailInput{
		Db: tx,

		Email: i.Email,
	})
	if err != nil {
		tx.Rollback()
		return errors.New("fail to get account")
	}

	if existentAccount == nil {
		createdAccount, err := serv.AccountRepository.Create(&models.CreateAccountInput{
			Db: tx,

			Email: i.Email,
		})
		if err != nil {
			tx.Rollback()
			return errors.New("fail to create account")
		}

		accountId = createdAccount.Id
		isFirstAccess = true
	} else {
		accountId = existentAccount.AccountId
	}

	magicLinkCode, err := serv.MagicLinkCodeRepository.Upsert(&models.UpsertMagicLinkRefreshTokenInput{
		Db: tx,

		AccountId:     accountId,
		IsFirstAccess: isFirstAccess,
	})
	if err != nil {
		tx.Rollback()
		return errors.New("fail to create account")
	}

	err = serv.EmailAdapter.SendVerificationCodeEmail(&adapters.SendVerificationCodeEmailInput{
		To:   i.Email,
		Code: magicLinkCode.Code,
	})
	if err != nil {
		tx.Rollback()
		return errors.New("fail to send sms")
	}

	tx.Commit()

	return nil
}

func (serv *AccountService) CreateFromPhoneProvider(i *models.CreateAccountFromPhoneInput) error {
	tx, err := serv.Db.Begin()
	if err != nil {
		return errors.New("fail to create transaction")
	}

	var accountId string
	var isFirstAccess bool

	existentAccount, err := serv.AccountRepository.GetByPhone(&models.GetAccountByPhoneInput{
		Db: tx,

		CountryCode: i.Phone.CountryCode,
		Number:      i.Phone.Number,
	})
	if err != nil {
		tx.Rollback()
		return errors.New("fail to get account")
	}

	if existentAccount == nil {
		createdAccount, err := serv.AccountRepository.Create(&models.CreateAccountInput{
			Db: tx,

			Phone: i.Phone,
		})
		if err != nil {
			tx.Rollback()
			return errors.New("fail to create account")
		}

		accountId = createdAccount.Id
		isFirstAccess = true
	} else {
		accountId = existentAccount.AccountId
	}

	magicLinkCode, err := serv.MagicLinkCodeRepository.Upsert(&models.UpsertMagicLinkRefreshTokenInput{
		Db: tx,

		AccountId:     accountId,
		IsFirstAccess: isFirstAccess,
	})
	if err != nil {
		tx.Rollback()
		return errors.New("fail to create account")
	}

	err = serv.SmsAdapter.SendVerificationCodeSms(&adapters.SendVerificationCodeSmsInput{
		To:   i.Phone.CountryCode + i.Phone.Number,
		Code: magicLinkCode.Code,
	})
	if err != nil {
		tx.Rollback()
		return errors.New("fail to send sms")
	}

	tx.Commit()

	return nil
}

func (serv *AccountService) ExchangeCode(i *models.ExchangeAccountCodeInput) (*models.AuthOutput, error) {
	tx, err := serv.Db.Begin()
	if err != nil {
		return nil, errors.New("fail to create transaction")
	}

	magicLinkCode, err := serv.MagicLinkCodeRepository.Get(&models.GetMagicLinkRefreshTokenInput{
		Db: tx,

		AccountId: i.AccountId,
		Code:      i.Code,
	})
	if err != nil {
		tx.Rollback()
		return nil, errors.New("fail to get account")
	}

	if magicLinkCode == nil {
		tx.Rollback()
		return nil, errors.New("magic link code doesn't exist")
	}

	tx.Commit()

	return serv.genAuthOutput(&genAuthOutputInput{
		db: tx,

		accountId:     i.AccountId,
		isFirstAccess: magicLinkCode.IsFirstAccess,
		refresh:       true,
	})
}

func (serv *AccountService) RefreshToken(i *models.RefreshAccountTokenInput) (*models.RefreshAccountTokenOutput, error) {
	tx, err := serv.Db.Begin()
	if err != nil {
		return nil, errors.New("fail to create transaction")
	}

	refreshToken, err := serv.RefreshTokenRepository.Get(&models.GetRefreshTokenInput{
		Db: tx,

		AccountId:    i.AccountId,
		RefreshToken: i.RefreshToken,
	})
	if err != nil {
		tx.Rollback()
		return nil, errors.New("fail to get account")
	}

	if !refreshToken {
		tx.Rollback()
		return nil, errors.New("refresh token doesn't exist")
	}

	accessToken, err := serv.TokenAdapter.GenAccess(&adapters.GenAccessInput{
		AccountId: i.AccountId,
	})
	if err != nil {
		tx.Rollback()
		return nil, errors.New("fail to generate access token")
	}

	tx.Commit()

	return &models.RefreshAccountTokenOutput{
		AccessToken: accessToken.AccessToken,
		ExpiresAt:   accessToken.ExpiresAt,
	}, nil
}
