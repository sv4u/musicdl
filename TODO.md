# `musicdl` Development To Do's

- [ ] [Make a slim version of the Docker image (make slim the default)](./.cursor/plans/docker_slim_image.plan.md)
- [ ] [Parallelize queries and downloads](./.cursor/plans/parallelize_queries_downloads.plan.md)
- [ ] [Add caching](./.cursor/plans/add_caching.plan.md)
- [ ] [Refactor into plan architecture](./.cursor/plans/plan_architecture_refactor.plan.md)
  - Instead of sequentially going through configuration, read through the entire configuration and create a download "plan"
  - The download "plan" is a list of all items that need to be downloaded based on the entire configuration
  - The download "plan" initial will have duplicate elements but will go through an optimization step before it is executed
- [ ] [Properly handle `spotipy` rate/request limit](./.cursor/plans/spotipy_rate_limiting.plan.md)
- [x] [Reimplement `spotdl` library and simplify complexity of it](./.cursor/plans/re-implement_spotdl_for_musicdl.plan.md)
- [ ] [Update changelog generation to be more structured](./.cursor/plans/structured_changelog_generation.plan.md)
