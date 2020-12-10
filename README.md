Go Redis Leaderboard
===========

A leaderboard written in [Go](http://golang.org/) using [Redis](http://redis.io/) database.

Base idea is taken from [this](https://github.com/dayvson/go-leaderboard) project. But this project uses `go-redis` 
package and supports saving of generic user info to redis.

Features
--------

* Create multiple Leaderboards by name 
* You can rank a member the leaderboard will be updated automatically
* Remove a member from a specific Leaderboard
* Get total of users in a specific Leaderboard and also how many pages it has.
* Get leaders on any page

How to use
----------

Create a new leaderboard or attach to an existing leaderboard named 'awesome_leaderboard': 
<pre>
    const UserInfoBucket = "users_info_bucket"
    
    redisSettings := redisLeaderboard.RedisSettings{
        Host:     "127.0.0.1:6379",
        Password: "",
        DB:       0,
    }
    
    awesomeLeaderboard, err := redisLeaderboard.NewLeaderboard(redisSettings, redisLeaderboard.ProdMode, "awesome_leaderboard", UserInfoBucket, redisLeaderboard.DefaultPageSize)
    //return an awesomeLeaderboard
</pre>  

Adding or getting member from awesome_leaderboard using FirstOrInsertMember(userID, score):
<pre>
    awesomeLeaderboard.FirstOrInsertMember("12345", 33)
    awesomeLeaderboard.FirstOrInsertMember("45678", 44)
    awesomeLeaderboard.FirstOrInsertMember("111", 12)
</pre>

You can call IncrementMemberScore(userID, incrementBy) with the same member and the leaderboard will be updated automatically:
<pre>
	awesomeLeaderboard.IncrementMemberScore("12345", 7481523)
	//return an user: User{UserID:"12345", Score:7481523, Rank:1}
</pre>

Getting a total number of members on awesome_leaderboard using TotalMembers():
<pre>
	awesomeLeaderboard.TotalMembers()
	//return an int: 3
</pre>

Getting the member and his info using GetMember(userID, withInfo):
<pre>
	awesomeLeaderboard.GetMember("12345", true)
	//return 
	    {
                 "user_id": "12345",
                 "score": 30,
                 "rank": 1,
                 "additional_info": {
                     "user_name": "Jane Doe | 2020",
                     "email": "jane@doe.com"
                 }
            }
</pre>

Getting leaders using GetLeaders(page):
<pre>
	awesomeLeaderboard.GetLeaders(1)
	//return an array of users with highest score in a first page (you can specify any page): [pageSize]User
</pre>

Installation
------------

Install Go Redis Leaderboard using the "go get" command:

    go get github.com/croatiangrn/go-redis-leaderboard


Dependencies
------------
* Go language distribution
* Redis client for Golang (github.com/go-redis/redis/v8)



Contributing
------------

* Contributions are welcome.
* Take care to maintain the existing coding style.
* Open a pull request


License
-------
Released under the [Apache License](LICENSE).