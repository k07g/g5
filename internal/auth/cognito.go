package auth

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
)

// CognitoVerifier resolves access tokens by asking Amazon Cognito who they
// belong to. Cognito's GetUser operation only needs the access token
// itself (Cognito validates its own signature), so no user pool or app
// client configuration is required here — this service just needs to be
// pointed at the right region.
type CognitoVerifier struct {
	client *cognitoidentityprovider.Client
}

func NewCognitoVerifier(ctx context.Context, region string) (*CognitoVerifier, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return &CognitoVerifier{
		client: cognitoidentityprovider.NewFromConfig(cfg),
	}, nil
}

var _ Verifier = (*CognitoVerifier)(nil)

func (c *CognitoVerifier) VerifyToken(ctx context.Context, accessToken string) (*Identity, error) {
	out, err := c.client.GetUser(ctx, &cognitoidentityprovider.GetUserInput{
		AccessToken: aws.String(accessToken),
	})
	if err != nil {
		return nil, err
	}

	identity := &Identity{}
	for _, attr := range out.UserAttributes {
		switch aws.ToString(attr.Name) {
		case "sub":
			identity.Sub = aws.ToString(attr.Value)
		case "email":
			identity.Email = aws.ToString(attr.Value)
		}
	}
	return identity, nil
}
