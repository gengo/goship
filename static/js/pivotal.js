(function($) {
  var config = {
    selectors: { // according to query selector standards
      project: '.project',
      project_id: 'data-id',
      environment: '.environment',
      story_column: '.story',
      github_link: '.GitHubDiffURL',
      refresh_button: '.refresh',
    },
    github: {
      url_compare_regexp: /.*\/(.*)\/compare/
    },
    urls: {
      pivotal: {
        base: 'https://www.pivotaltracker.com',
        story: '/story/show',
        api: '/services/v5',
      }
    }
  };

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

    $(config.selectors.project).each(function() {
      var $this = $(this);
      var project = $this.data('id');
      var url = $this.find(config.selectors.github_link).attr('href');

      if (url) {
        diffURLs[project] = url;
      }
    });

    return diffURLs;
  }

  /**
   * getLinkDiv return an HTML string of story link and status
   * @param  {Object} pt_info Pivotal story object
   * @return {String}         HTML string
   */
  function getLinkDiv(pt_info) {
    var links = [];

    for (f in pt_info) {
      var story = pt_info[f];
      var status = '<span class="label label-'+ mapStatusLabelClass(story.status) +'">'+ story.status +'</span>';
      var dep = '<span class="badge" data-toggle="popover" data-content="<li>'+ story.dependencies.join('</li><li>') +'</li>">'+ story.dependencies.length +'</span>';

      links.push('<a href="'+ story.url +'" target="_blank">#'+ story.id +'</a>'+ status + dep +'<br/>');
    }

    return links.join('');
  }

  function injectPivotalStatus(project_id, pt_info) {
    var project_selector = config.selectors.project + '[' + config.selectors.project_id + '="' + project_id + '"]';
    var $project = $(project_selector).find(config.selectors.story_column);
    $project.append(getLinkDiv(pt_info));
    $project.find('[data-toggle="popover"]').popover({
      html: true,
      trigger: 'hover',
      placement: 'top'
    });
  }

  /**
   * getGithubCommits returns Github commits for a given repository
   * @param  {String}   github_token Github token
   * @param  {String}   repo         Github repository name
   * @param  {String}   sha1         Current commit hash
   * @param  {String}   sha2         Latest commit hash
   * @param  {Function} callback
   */
  function getGithubCommits(github_token, repo, sha1, sha2, callback) {
    $.ajax({
      url: 'https://api.github.com/repos/gengo/'+ repo +'/compare/'+ sha1 +'...'+ sha2,
      type: 'GET',
      headers: {
        'Authorization': 'token '+ github_token
      },
      success: function(data) {
        callback(data.commits);
      },
      error: function(err) {
        console.error(err);
      }
    });
  }

  /**
   * getPivotalStoryIDs return an array of Pivotal story IDs from array of messages
   * @param  {Array} msgs Array of string messages
   * @return {Array}      Array of pivotal IDs
   */
  function getPivotalStoryIDs(msgs) {
    var storyIDs = msgs.map(function(str) {
      return parseInt(str.match(/\[#(\d+)\]/)[1], 10);
    });

    return removeDupesFromArray(storyIDs);
  }

  /**
   * getPivotalStoryInfo return pivotal story data
   * @param  {string}   pt_token Pivotal token
   * @param  {number}   story_id Story id
   * @param  {Function} callback
   */
  function getPivotalStoryInfo(pt_token, story_id, callback) {
    $.ajax({
      url: config.urls.pivotal.base + config.urls.pivotal.api + '/stories/' + story_id,
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
      url: config.urls.pivotal.base + config.urls.pivotal.api +'/projects/'+ project_id +'/stories/'+ story_id +'/comments',
      type: 'GET',
      headers: {
        'X-TrackerToken': pt_token
      },
      success: function(data) {
        // Find all git commit messages
        var list = data.filter(function(obj) {
          return obj.commit_type;
        });
        // Find repo name from comment body text
        list = list.map(function(item) {
          return item.text.split('https://github.com/gengo/')[1].split('/commit/')[0];
        });

        callback(removeDupesFromArray(list));
      }
    });
  }

  /**
   * showNoStoriesMessage
   * @param  {jQuery} $target jQuery button object
   */
  function showNoStoriesMessage($target) {
    $target.closest(config.selectors.story_column).text('No stories found.');
  }

  $(document).ready(function() {
    // add button to story columns
    var button = '<button class="btn btn-default getStories">Get stories</button><span class="loading" style="display:none">Loading...</span>';
    $(config.selectors.story_column).each(function() {
      $(this).html(button);
    });

    // When reset button clicked add Get stories button
    $(config.selectors.refresh_button).click(function() {
      $(this).closest(config.selectors.project).find(config.selectors.story_column).html(button);
    });

    $('.project').on('click', '.getStories', function(e) {
      e.preventDefault();

      var $this_button = $(e.currentTarget);
      var project = $this_button.parents(config.selectors.project).data('id');
      var diffs = getProjectDiffs();

      $this_button.hide(); // do not show button

      if (project in diffs) {
        // great! we found diffs for this project; load the pivotal stories
        $this_button.siblings('.loading').show();

        (function(project) {
          var url = diffs[project];
          // hashes: currentCommit...latestCommit
          var commitHashes = (url.substr(url.lastIndexOf('/') + 1)).split('...');
          var repositoryName = url.match(config.github.url_compare_regexp)[1];

          getGithubCommits(GITHUB_TOKEN, repositoryName, commitHashes[0], commitHashes[1], function(commits) {
            // Array of comitt messages
            var messages = commits.map(function(obj) {
              return obj.commit.message;
            });
            // Array of pivotal story IDs
            var pivotal_ids = getPivotalStoryIDs(messages);
            if (!pivotal_ids) {
              showNoStoriesMessage($this_button);
              return;
            };

            $this_button.siblings('.loading').hide();

            var display_message = [];
            for (pt_id in pivotal_ids) {
              var id = pivotal_ids[pt_id];

              getPivotalStoryInfo(PIVOTAL_TOKEN, id, function(info) {
                display_message[id] = info;
                injectPivotalStatus(project, display_message);
              });
            }
          });
        })(project);
      }
      else {
        showNoStoriesMessage($this_button);
      }
    });

  });
}(jQuery));
