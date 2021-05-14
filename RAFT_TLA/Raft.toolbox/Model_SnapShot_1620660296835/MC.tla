---- MODULE MC ----
EXTENDS raft, TLC

\* CONSTANT definitions @modelParameterConstants:0AppendEntriesResponse
const_162067048159132000 == 
{}
----

\* CONSTANT definitions @modelParameterConstants:1Follower
const_162067048159133000 == 
0
----

\* CONSTANT definitions @modelParameterConstants:2Leader
const_162067048159134000 == 
2
----

\* CONSTANT definitions @modelParameterConstants:3Nil
const_162067048159135000 == 
50
----

\* CONSTANT definitions @modelParameterConstants:4RequestVoteResponse
const_162067048159136000 == 
{}
----

\* CONSTANT definitions @modelParameterConstants:5Candidate
const_162067048159137000 == 
1
----

\* CONSTANT definitions @modelParameterConstants:6RequestVoteRequest
const_162067048159138000 == 
{}
----

\* CONSTANT definitions @modelParameterConstants:7AppendEntriesRequest
const_162067048159139000 == 
{}
----

\* CONSTANT definitions @modelParameterConstants:8Value
const_162067048159140000 == 
10
----

\* CONSTANT definitions @modelParameterConstants:9Server
const_162067048159141000 == 
{1,2,3,4,5}
----

=============================================================================
\* Modification History
\* Created Mon May 10 13:14:41 CDT 2021 by joshpapermaster
