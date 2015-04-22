(function($){
  var undefined;

  $(function(){
    $('[data-toggle="tooltip"]').tooltip();
    // make ajax queries for each project
    $('.project').each(function(){
        refreshProject(this);
    });
  });

  $('.refresh').click(function(e) {
    refreshProject($(this).closest('.project'));
    e.preventDefault();
  });

  function refreshProject(project) {
      var $hostSkeleton = $('#host-skeleton');
      var $project = $(project),
      projectId = $project.data('id');
      $project.find('.hosts').text('Loading...');
      $project.find('.form-deploy').addClass('hidden');
      $.ajax({
        type: 'GET',
        url: '/commits/' + projectId,
        dataType: 'json',
        success: function(response) {
          var environments = response.Environments,
            project = response.Name,
            $project = $('[data-id="' + project +'"]');

          for (var e = 0; e < environments.length; e++) {
            var env = environments[e];
            var $env = $project.find('.environment[data-id="'+ env.Name +'"]');
            var $hosts = $env.find('.hosts');
            $hosts.text('');

            for (var h = 0; h < env.Hosts.length; h++) {
              var host = env.Hosts[h];
              var $host = $hostSkeleton.clone().removeAttr('id').removeClass('hidden');
              $host.find('.GitHubCommitURL').attr({
                'href': host.GitHubCommitURL
              }).text(host.ShortCommitHash);
              $hosts.append($host);
              if (host.GitHubDiffURL) {
                $host.find('.GitHubDiffURL').attr('href', host.GitHubDiffURL).closest('span.hidden').removeClass('hidden');
              }
            }
            if (env.IsDeployable) {
              for (var h = 0; h < env.Hosts.length; h++) {
                var host = env.Hosts[h];
                if (host.GitHubDiffURL) {
                  $deployForm = $env.find('.form-deploy');
                  $deployForm.removeClass('hidden').find('[name="diffUrl"]').val(host.GitHubDiffURL);
                  $deployForm.find('[name="from_revision"]').val(host.LatestCommit);
                  $deployForm.find('[name="to_revision"]').val(env.LatestGitHubCommit);
                  break;
                }
              }
            }
            $comment = $env.find('.comment');
            if (env.Comment || env.IsLocked) {
              $env.find(".glyphicon-comment").removeClass('hidden').popover({
                trigger: 'hover focus',
                content: env.Comment,
                placement: 'left'
              });
            }
            if (env.IsLocked) {
              $deployForm = $env.find('.form-deploy').find('.btn');
              $deployForm.addClass('disabled');
            }
          }
        }
      });
  }

  // Extended disable function
  $.fn.extend({
      disable: function(state) {
          return this.each(function() {
              var $this = $(this);
              if($this.is('input, button'))
                this.disabled = state;
              else
                $this.toggleClass('disabled', state);
          });
      }
  });
}(jQuery));