(function($){
  var config = {
    selectors: { // according to query selector standards
      project: '.project',
      project_id: 'data-id',
      environment: '.environment',
      story_column: '.story',
      github_link: '.GitHubDiffURL',
      refresh_button: '.refresh',
    },
    pivotal: {
      label: '<span class="label label-unknown">status</span>',
      ticket_id_regexp: /\[#(\d+)\]/g  // we want all matches, not just the first ticket
    },
    github: {
      url_compare_regexp: /.*\/(.*)\/compare/
    },
    urls: {
      pivotal: {
        base: "https://www.pivotaltracker.com",
        story: '/story/show',
        api: '/services/v5',
      }
    }
  };

  function mapStatusLabelClass(status) {
    // returns label class type (as of Twitter Bootstrap 3)
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

  function getProjectDiffs(){

      var project_diffs = [];

      $(config.selectors.project).each(function() {
          var project = $(this).attr(config.selectors.project_id);
          var url = $(this).find(config.selectors.github_link).attr('href');
          if (url && url != "undefined"){
              project_diffs[project] = url;
          }
      });
      return project_diffs;
  }

  function getLinkDiv(pt_info) {
    var links = '',
        info,
        status_label;
    for(f in pt_info)
    {
      info = pt_info[f];
      status_label = config.pivotal.label.replace('unknown' , mapStatusLabelClass(info['status'])).replace('status', info['status']);
      links += '<a href="' + info['url'] + '" target="_blank">#' + info['id'] + '</a> ' + status_label + '<br/>';
    }
    return links;
  }

  function injectPivotalStatus(project_id, pt_info) {
      var project_selector = config.selectors.project + '[' + config.selectors.project_id + '="' + project_id + '"]';
      $(project_selector).find(config.selectors.story_column).append(getLinkDiv(pt_info));
  }

  function getGithubPivotalIDs(github_token, repo, sha1, sha2, callback){
    var compare_uri = 'https://api.github.com/repos/gengo/'+ repo + '/compare/' + sha1 + '...' + sha2;
    $.ajax({
      url: compare_uri,
      type: 'GET',
      beforeSend: function(xhr) { 
        xhr.setRequestHeader("Authorization", "token " + github_token);
      },
      success: function (data) {
        callback(data.commits);            
      },
      error: function (data) {
        console.log(data);
      }
    });
  }

  function stripBrackets(ticket_id_str){
    return parseInt(ticket_id_str.match(/\[#(\d+)\]/)[1], 10); // base 10
  }

  function getCommitIDs(msgs){
    // create an object with no prior properties; therefore, all keys in set are the values we want
    pivotal_ids = Object.create(null);
    for(msg in msgs)
    {
      var message = msgs[msg];
      var matches = message.match(config.pivotal.ticket_id_regexp);
      if(!matches) continue;
      matches.forEach(function(ticket, i, arr){
        var id = stripBrackets(ticket);
        if(id && !(id in pivotal_ids)) pivotal_ids[id] = true;
      });
    }
    return Object.keys(pivotal_ids); // returning just the keys gives us all unique pivotal IDs
  }

  function getPivotalInfo(pt_token, story_id, callback){
    var story_uri = config.urls.pivotal.base + config.urls.pivotal.api + '/stories/' + story_id;
    $.ajax({
      type:"GET",
      beforeSend: function (request)
      {
        request.setRequestHeader("X-TrackerToken", pt_token);
      },
      url: story_uri,
      success: function (data) {
        var info = {
          'id': story_id
        };
        info['status'] = data['current_state'];
        info['url'] = data['url'];
        callback(info);
      }
    });
  }

  var diffs = getProjectDiffs();
  setTimeout(function(){ diffs = getProjectDiffs(); }, 1000);  // fetch diffs again after 1 sec
  $(document).ready(function(){

    // add button to story columns
    var button = '<button class="btn btn-default getStories">Get stories</button>';
    $(config.selectors.story_column).each(function(){
      $(this).html(button);
    });


    // set up listeners on Refresh Button
    $(config.selectors.refresh_button).click(function(){
      // display button once more
      $(this).parents(config.selectors.project).find(config.selectors.story_column).html(button);
      $(this).parents(config.selectors.project).find('.getStories').click(pivotalButtonOnClickHandler);
    });

    // "get stories" button onClick handler
    $('.getStories').click(pivotalButtonOnClickHandler);

    function pivotalButtonOnClickHandler(e) {
      var $this_button = $(e.currentTarget);
      e.preventDefault();
      diffs = getProjectDiffs();
      var project = $this_button.parents(config.selectors.project).data('id');
      var failure_message = 'No stories found.';
      $this_button.hide(); // do not show button
      if(project in diffs) {
        // great! we found diffs for this project; load the pivotal stories
        (function(project) {
        var url = diffs[project];
        // hashes: currentCommit...latestCommit
        var hashes = (url.substr(url.lastIndexOf('/') + 1)).split('...');
        var proj_name = url.match(config.github.url_compare_regexp)[1];

        getGithubPivotalIDs(GITHUB_TOKEN, proj_name, hashes[0], hashes[1], function(commits){
          var messages = [];
          for(commit in commits) {
            messages.push(commits[commit]['commit']['message']); // get all github commit messages
          }

          var pivotal_ids = getCommitIDs(messages); // extract pivotal ticket IDs
          if(!pivotal_ids) {
            $this_button.parents(config.selectors.story_column).text(failure_message);
            return
          };

          var display_message = [];
          for(pt_id in pivotal_ids)
          {
            var id = pivotal_ids[pt_id];
            getPivotalInfo(PIVOTAL_TOKEN, id, function(info){
              display_message[id] = info;
              injectPivotalStatus(project, display_message);
            });
          }
        });
      })(project);
      }
      else {
        $this_button.parents(config.selectors.story_column).text(failure_message);
      }
    }
  });
}(jQuery));
