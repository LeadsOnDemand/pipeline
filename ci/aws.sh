cd `dirname $0`

GitBranch=`git branch | grep \* | cut -d ' ' -f2`
GitRepo=`git rev-parse --show-toplevel | rev | cut -d/ -f1 | rev`
GitOwner='LeadsOnDemand'
## some resource require lowercase so please no uppercase
StackName=`echo "$GitRepo-$GitBranch" | tr '[:upper:]' '[:lower:]'`
## parameters


echo Branch - $GitBranch
echo Repo - $GitRepo
echo Owner - $GitOwner
echo StackName - $StackName

aws cloudformation deploy \
--template-file cf.yml \
--capabilities CAPABILITY_NAMED_IAM \
--stack-name $StackName \
# --parameter-overrides GitRepo=$GitRepo \
# GitBranch=$GitBranch \
# GitOwner=$GitOwner \
