# Current Implementation of Authoriation

### User
I have maintained a seperate contract under domain named as user.go which consists of all the user profile information
It consists of :
ID
Github ID
Username 
Email (Optional if the org is private)
AvatarURL
CreatedAT

And then storing the information in the Postgres Database 

### Auth
Here it handles the two step process into one step where the github oauth is managed along with the callback to installtion id to install the github app for a user

Also we ensure to generate two session tokens access token and refresh token and do no rely on the github app access token for the user session management 

Also a logout feature to allow the user to log out from the registred session 

### Github

Here we are storing the follwing infromation when the github app is installeed and after installtion there is the following attributes we are taking care of : 
ID			
UserID (for a User type)	
InstalltionID	
AccountName	

And then storing the information in the Postgres Database

## 1 April 2026 

Today I have addeed repository functions here for the defined repositories in the domain defined for the user and the github these have been defined here

Now there is the following things you should know before proceeding to the next part : 

## The documentation needed for reference here is given below : 
https://go.dev/doc/database/manage-connections#dedicated_connections
https://go.dev/doc/database/querying
https://pkg.go.dev/database/sql

These are the main docs used for implementing the queries , dedicated Connection and then these are implemented as per given in the Domain

### In authorization/user/repository/pgsql/pgsql_user.go

There is the detected connection of the database there is the connection pipeline set up for the completetion of the queries and there connection pool is managed by Open and then we use Conn for dedicated detection then there are the following the below functions : 

GetById 
To get the user by id and return User,error

GetByUsername
To get the user by usernamr and return User,error

Store 
Insert the values User in the pg db and then return err

### In authorization/github/repository/pgsql/pgsql_github.go

There is the detected connection of the database there is the connection pipeline set up for the completetion of the queries and there connection pool is managed by Open and then we use Conn for dedicated detection then there are the following the below functions : 

StoreInstallation 
To store the installation id for the installed github app and then return error

GetInstallationByUserID 
Here it returns a unique row then there is has the userId as params and returns the GithubInstllation