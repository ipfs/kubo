# Guidelines to create great commit messages


We use [GitCop](https://gitcop.com) to check that commit messages are
properly written. The rules are the following:

* The first line of a commit message, called the subject line should
  not be more than 80 characters long.

* The commit message should end with the following trailers:

  ```
  Licence: MIT
  Signed-off-by: User Name <email@address>
  ```

  where "User Name" is the author's real name and email@address one of
  the author's valid email addresses.

  These trailers mean that the author agrees with the following
  document (which comes from http://developercertificate.org/):

  [developer-certificate-of-origin](./developer-certificate-of-origin)

  and with licensing the work under the MIT license available in the
  following file:

  [LICENSE](./LICENSE)

  To help you automatically add these trailers, you can run the
  following script:

  [setup_commit_msg_hook.sh](./setup_commit_msg_hook.sh)

  which will setup a Git commit-msg hook that will add the above
  trailers to all the commit messages you write.
