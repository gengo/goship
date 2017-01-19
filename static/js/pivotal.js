(function($) {
  var config = {
    environment: $('.environment').eq(0).data('id'),
    urls: {
      pivotal: {
        api: 'https://www.pivotaltracker.com/services/v5',
      }
    }
  };

  // register common error handler on Ajax errors / failures
  $(document).ajaxError(console.error.bind(console));

  /**
   * removeDupesFromArray returns an array without dupes
   * @param  {Array} arr Any array
   * @return {Array}     Array without dups
   */
  function removeDupesFromArray(arr) {
    return arr.filter(function(elem, i) {
      return arr.indexOf(elem) == i;
    });
  }

  /**
   * isStringInArray return a boolean if item exists in an Array
   * @param  {String}  value  Some string
   * @param  {Array}   array  Array of strings
   * @return {Boolean}
   */
  function isStringInArray(value, array) {
    return array.indexOf(value) > -1;
  }

  /**
   * groupBy is a similar method as in underscore groupBy
   * @param  {Array}   list      Array of items
   * @param  {Function} callback Call back with context
   * @return {Object}            Object with the result
   */
  function groupBy(list, callback) {
    return list.reduce(function(result, current) {
      var key = callback(current);

      if (key) {
        result[key] = result[key] || [];
        result[key].push(current);
      }

      return result;
    }, {});
  }

  /**
   * mapStatusLabelClass returns label class type (as of Twitter Bootstrap 3)
   * @param  {string} status Pivotal story status
   * @return {string}        Bootstrap class name
   */
  function mapStatusLabelClass(status) {
    switch(status) {
      case 'rejected':
        return 'danger';
      case 'accepted':
        return 'success';
      case 'delivered':
        return 'warning';
      case 'finished':
        return 'primary';
      case 'started':
        return 'info';
      default:
        return 'default';
    }
  }

  /**
   * getProjectDiffs return an Object, project name as the key and github diff URLs
   * as the value
   * @return {Object} {project_name: diff_url}
   */
  function getProjectDiffs() {
    var diffURLs = {};

    $('.project').each(function() {
      var $this = $(this);
      var project = $this.data('id');
      var url = $this.find('.GitHubDiffURL').attr('href');

      if (url) {
        diffURLs[project] = url;
      }
    });

    return diffURLs;
  }

  /**
   * getGithubCommits returns Github commits for a given repository
   * @param  {String}   github_token Github token
   * @param  {String}   repo         Github repository name
   * @param  {String}   commitHashe  Current and Latest commit hash
   * @param  {Function} callback
   */
  function getGithubCommits(github_token, repo, commitHashe, callback) {
    $.ajax({
      url: 'https://api.github.com/repos/gengo/'+ repo +'/compare/'+ commitHashe,
      type: 'GET',
      headers: {
        'Authorization': 'token '+ github_token
      },
      success: function(data) {
        callback(data.commits);
      }
    });
  }

  /**
   * getPivotalStoryIDs return an array of Pivotal story IDs from array of messages
   * @param  {Array} msgs Array of string messages
   * @return {Array}      Array of pivotal IDs
   */
  function getPivotalStoryIDs(msgs) {
    var reg = /\[#(\d+)\]/; // Pivotal story ID, ex) [#12345]
    var storyIDs = msgs
      .filter(function(str) {
        return reg.test(str);
      })
      .map(function(str) {
        return parseInt(str.match(reg)[1], 10);
      });

    return removeDupesFromArray(storyIDs);
  }

  /**
   * getPivotalStoryIDs return true if there is a commit without pivotal story
   * @param  {Array}   msgs Array of string messages
   * @return {Boolean}
   */
  function hasCommitWithoutStory(msgs) {
    var reg = /\[#(\d+)\]/; // Pivotal story ID, ex) [#12345]
    for (var i = 0; i < msgs.length; i++) {
      if (!reg.test(msgs[i])) {
        return true;
      }
    }

    return false;
  }

  /**
   * hasSqlMigration return true if there is a sql-migrations repo dependency
   * @param  {Array}   storyList Array of stories
   * @return {Boolean}
   */
  function hasSqlMigration(storyList) {
    for (var i = 0; i < storyList.length; i++) {
      dependencies = storyList[i].dependencies.all;
      for (var d = 0; d < dependencies.length; d++) {
        dependency = dependencies[d];
        if (dependency === 'sql-migrations') {
          return true
        }
      }
    }

    return false
  }

  /**
   * getPivotalStoryInfo return pivotal story data
   * @param  {string}   pt_token Pivotal token
   * @param  {number}   story_id Story id
   * @param  {Function} callback
   */
  function getPivotalStoryInfo(pt_token, story_id, callback) {
    $.ajax({
      url: config.urls.pivotal.api + '/stories/' + story_id,
      type: 'GET',
      headers: {
        'X-TrackerToken': pt_token
      },
      success: function(data) {
        getRepoDependencies(pt_token, data.project_id, story_id, function(list) {
          callback({
            id: story_id,
            url: data.url,
            status: data.current_state,
            dependencies: list
          });
        });
      },
      error: function(err) {
        // e.g., no pivotal story found
        callback(undefined);
      }
    });
  }

  /**
   * getRepoDependencies return a list of dependencies for stroy in a project
   * @param  {String}   pt_token   Pivotal token
   * @param  {number}   project_id Project ID
   * @param  {number}   story_id   Story ID
   * @param  {Function} callback   Returns Pivotal story ID list
   */
  function getRepoDependencies(pt_token, project_id, story_id, callback) {
    $.ajax({
      url: config.urls.pivotal.api + '/projects/' + project_id + '/stories/' + story_id + '/comments',
      type: 'GET',
      headers: {
        'X-TrackerToken': pt_token
      },
      success: function(data) {
        var PULL_REQUEST_REGEX = /Merge pull request/;
        var COMMIT_REPO_REGEX = /https:\/\/github.com\/gengo\/(.+)\/commit\//;
        var DEPLOY_REPO_REGEX = new RegExp('Deployed (.+) to '+ config.environment +': ');

        var activities = data.filter(function(activity) {
          return activity.commit_type === 'github' || DEPLOY_REPO_REGEX.test(activity.text);
        }).reverse();

        var activitiesByRepo = groupBy(activities, function(activity) {
          // With commit message
          if (COMMIT_REPO_REGEX.test(activity.text)) {
            return activity.text.match(COMMIT_REPO_REGEX)[1];
          }
          // Deployed message
          if (DEPLOY_REPO_REGEX.test(activity.text)) {
            return activity.text.match(DEPLOY_REPO_REGEX)[1];
          }
        });

        var inProgress = [];
        var readyToDeploy = [];
        var deployed = [];
        for (repo in activitiesByRepo) {
          var activity = activitiesByRepo[repo][0];

          // Merged repo
          if (PULL_REQUEST_REGEX.test(activity.text)) {
            readyToDeploy.push(repo);
          }
          // In Progress repo
          else if (COMMIT_REPO_REGEX.test(activity.text)) {
            inProgress.push(repo);
          }
          // Deployed repo
          if (DEPLOY_REPO_REGEX.test(activity.text)) {
            deployed.push(repo);
          }
        }

        callback({
          'all': inProgress.concat(readyToDeploy, deployed),
          'in_progress': inProgress,
          'ready_to_deploy': readyToDeploy,
          'deployed': deployed
        });
      }
    });
  }

  /**
   * getPopoverHTML return a HTML string
   * @param  {Array} list Array of strings
   * @return {String}     HTML text string
   */
  function getPopoverHTML(dependencies) {
    var inProgress = dependencies.in_progress.map(function(repo) {
      return '<p><span class=\'label label-default\'>'+ repo +'</span></p>';
    }).join('');

    var readyToDeploy = dependencies.ready_to_deploy.map(function(repo) {
      return '<p><span class=\'label label-success\'>'+ repo +'</span></p>';
    }).join('');

    var deployed = dependencies.deployed.map(function(repo) {
      return '<p><s><span class=\'label label-info\'>'+ repo +'</span></s></p>';
    }).join('');

    return inProgress + readyToDeploy + deployed;
  }

  /**
   * onGetPivotalStoryInfoComplete is triggered when pitotal info loading completted,
   * and building up HTML to insert into the Story block.
   * @param  {String} project   GitHub project name
   * @param  {Array} storyList  Pivotal story info array
   */
  function onGetPivotalStoryInfoComplete(project, storyList) {
    var html = storyList.map(function(story) {
      return '<div> \
                <a href="'+ story.url +'" target="_blank">#'+ story.id +'</a> \
                &nbsp; \
                <span class="label label-'+ mapStatusLabelClass(story.status) +'">'+ story.status +'</span> \
                &nbsp; \
                <span class="badge" data-toggle="popover" data-content="'+ getPopoverHTML(story.dependencies) +'">'+ story.dependencies.all.length +'</span> \
             </div>';
    });

    $('.project[data-id="'+ project +'"]')
      .find('.story')
        .html(html)
      .find('[data-toggle="popover"]')
        .popover({
          html: true,
          trigger: 'hover',
          placement: 'top'
        });
  }

  /**
   * showNoStoriesMessage
   * @param  {jQuery} $target jQuery button object
   */
  function showNoStoriesMessage($target) {
    $target.closest('.story').text('No stories found.');
  }

  /**
   * showCommitWithoutStoryMessage
   * @param  {jQuery} $target jQuery button object
   */
  function showCommitWithoutStoryMessage($target) {
    $target.append('<div>Found commit without story.</div>');
  }

  /**
   * showSqlMigrationWarning
   * @param  {jQuery} $target jQuery button object
   */
  function showSqlMigrationMessage($target) {
    $target.append('<div>Found SQL-migrations.</div>');
  }

  $(document).ready(function() {
    // add button to story columns
    var button = '<button class="btn btn-default getStories">Get stories</button><span class="loading" style="display:none">Loading...</span>';
    $('.project .story').each(function() {
      $(this).html(button);
    });

    // When reset button clicked add Get stories button
    $('.refresh').click(function() {
      $(this).closest('.project').find('.story').html(button);
    });

    $('.project').on('click', '.getStories', function(e) {
      e.preventDefault();

      var $this_button = $(e.currentTarget);
      var $this_story = $this_button.closest('.story')
      var project = $this_button.parents('.project').data('id');
      var diffs = getProjectDiffs();

      $this_button.hide();

      if (project in diffs) {
        $this_button.siblings('.loading').show();

        var url = diffs[project];
        // hashes: currentCommit...latestCommit
        var commitHashe = url.substr(url.lastIndexOf('/') + 1);
        var repositoryName = url.match(/.*\/(.*)\/compare/)[1];

        getGithubCommits(GITHUB_TOKEN, repositoryName, commitHashe, function(commits) {
          // Array of comitt messages
          var messages = commits.map(function(obj) {
            return obj.commit.message;
          });
          // Array of pivotal story IDs
          var pivotal_ids = getPivotalStoryIDs(messages);
          if (pivotal_ids.length === 0) {
            showNoStoriesMessage($this_button);
            if (hasCommitWithoutStory(messages)) {
              showCommitWithoutStoryMessage($this_story);
            }
            return;
          };

          var storyList = [];
          for (var i = 0, imax = pivotal_ids.length; i < imax; i++) {
            getPivotalStoryInfo(PIVOTAL_TOKEN, pivotal_ids[i], function(info) {
              // if getPivtoalStoryInfo is unsuccessful, info is undefined
              !!info && storyList.push(info);
              // we have cycled through all possible pivotal stories (some unsuccessful perhaps)
              if (i === imax) {
                $this_button.siblings('.loading').hide();
                onGetPivotalStoryInfoComplete(project, storyList);
                if (hasCommitWithoutStory(messages)) {
                  showCommitWithoutStoryMessage($this_story);
                }
                if(hasSqlMigration(storyList)) {
                  showSqlMigrationMessage($this_story);
                }
              }
            });
          }
        });
      }
      else {
        showNoStoriesMessage($this_button);
      }
    });

  });
}(jQuery));
