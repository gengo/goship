(function($) {
  var config = {
    urls: {
      pivotal: {
        api: 'https://www.pivotaltracker.com/services/v5',
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
    var storyIDs = msgs
      .filter(function(str) {
        return /\[#(\d+)\]/.test(str);
      })
      .map(function(str) {
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
                <span class="badge" data-toggle="popover" data-content="<li>'+ story.dependencies.join('</li><li>') +'</li>">'+ story.dependencies.length +'</span> \
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
            return;
          };

          var storyList = [];
          for (var i = 0, imax = pivotal_ids.length; i < imax; i++) {
            getPivotalStoryInfo(PIVOTAL_TOKEN, pivotal_ids[i], function(info) {
              storyList.push(info);

              if (storyList.length === imax) {
                $this_button.siblings('.loading').hide();
                onGetPivotalStoryInfoComplete(project, storyList);
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
