#!/usr/bin/ruby

require 'set'

# Get commits in one branch, but not in another, accounting for cherry-picks.
# Accepts two arguments: base branch and old branch. Commits in base branch that
# are not in old branch will be reported.

# Preface: I know exactly enough ruby to be dangerous with it.
# For anyone reading this who is actually skilled at writing Ruby, I can only
# say I'm very, very sorry.

# Utility functions:

# Check if a given Git branch exists
def CheckBranchExists(branch)
  return `git branch --list #{branch}`.rstrip.empty?
end

# Returns author (email) and commit subject for the given hash
def GetCommitInfo(hash)
  info = `git log -n 1 --format='%ae%n%s' #{hash}`.split("\n")
  if info.length != 2
    puts("Badly-formatted commit with hash #{hash}")
    exit(127)
  end
  return info[0], info[1]
end

# Actual script begins here

if ARGV.length != 2
  puts("Must provide exactly 2 arguments, base branch and old branch")
  exit(127)
end

# Both branches must exist
ARGV.each do |branch|
  if !CheckBranchExists(branch)
    puts("Branch #{branch} does not exist")
    exit(127)
  end
end

base = ARGV[0]
old = ARGV[1]

# Get a base list of commits
commits = `git log --no-merges --format=%H #{base} ^#{old}`.split("\n")

# Alright, now for the hacky bit.
# We want to remove every commit with a shortlog precisely matching something in
# the old branch. This is an effort to catch cherry-picks, where commit ID has
# almost certainly changed because the committer is different (and possibly
# conflicts needed to be resolved).
# We will match also try and match author, but not committer (which is reset to
# whoever did the cherry-pick). We will *not* match full commit body - I
# routinely edit these when I fix cherry-pick conflicts to indicate that I made
# changes. A more ambitious future committer could attempt to see if the body of
# the commit message in the old branch is a subset of the full commit message
# from the base branch, but there are potential performance implications in that
# due to the size of the string comparison that would be needed.
# This will not catch commits where the shortlog is deliberately altered as part
# of the cherry pick... But we can just ask folks not to do that, I guess?
# (A classic example of something this wouldn't catch: cherry-picking a commit
# to a branch and then prepending the branch name to the commit subject. I see
# this a lot in Github PR subjects, but fortunately not much at all in actual
# commit subjects).

# Begin by fetching commit author + subject for each commit in old branch.
# Map each author to an array of potential commit subjects.
oldIndex = {}

# TODO: This could probably be made a whole lot more efficient by unifying the
# GetCommitInfo bits into two big `git log --format` calls.
# But I'm not really ambitious enough to do that...
oldCommits = `git log --no-merges --format=%H #{old}`.split("\n")
oldCommits.each do |hash|
  name, subject = GetCommitInfo(hash)
  if oldIndex[name] == nil
    oldIndex[name] = Set[]
  end
  oldIndex[name].add(subject)
end

# Go through our earlier commits list and check for matches.
filtered = commits.reject do |hash|
  name, subject = GetCommitInfo(hash)
  oldIndex[name] != nil && oldIndex[name].include?(subject)
end

# We have now filtered out all commits we want to filter.
# Now we just have to print all remaining commits.
# This breaks the default pager, but we can just pipe to less.
filtered.each do |hash|
  puts `git log -n 1 #{hash}`
  puts "\n"
end
