/* Quiz y poll — interacción LOCAL (issue #198).
 *
 * No hay backend y no se recolecta nada: el conteo del poll vive en la página
 * de quien la abre y desaparece al recargar. Se dice explícito porque la idea
 * de "encuesta" invita a suponer lo contrario.
 *
 * Se ata a las clases que documenta features/themes-styling.md — .option,
 * .option.correct, .explanation y .poll-option —
 * que son el contrato público con los temas. El HTML sin JS ya es válido: el
 * quiz impreso trae la respuesta marcada desde el servidor.
 */
(function () {
    'use strict';

    function revealQuiz(quiz, chosen) {
        var answer = parseInt(quiz.getAttribute('data-answer'), 10);
        var options = quiz.querySelectorAll('.slidelang-option');

        for (var i = 0; i < options.length; i++) {
            var index = parseInt(options[i].getAttribute('data-index'), 10);
            options[i].disabled = true;
            if (index === answer) {
                options[i].classList.add('slidelang-correct');
            } else if (options[i] === chosen) {
                options[i].classList.add('slidelang-incorrect');
            }
        }

        var explanation = quiz.querySelector('.slidelang-explanation');
        if (explanation) {
            explanation.hidden = false;
        }
    }

    function togglePoll(poll, chosen) {
        var multiple = poll.getAttribute('data-multiple') === 'true';
        var options = poll.querySelectorAll('.slidelang-poll-option');

        if (!multiple) {
            for (var i = 0; i < options.length; i++) {
                if (options[i] !== chosen) {
                    options[i].classList.remove('slidelang-selected');
                }
            }
        }
        chosen.classList.toggle('slidelang-selected');

        // Standalone SlideLang only owns local selection state. Aggregated
        // response counts belong to a host with a live channel (for example
        // SlideWeave), so the toolchain deliberately does not fabricate
        // result bars from one viewer's choices.
        for (var j = 0; j < options.length; j++) {
            options[j].setAttribute(
                'aria-pressed',
                options[j].classList.contains('slidelang-selected') ? 'true' : 'false'
            );
        }
    }

    function init() {
        document.addEventListener('click', function (event) {
            var option = event.target.closest ? event.target.closest('.slidelang-option, .slidelang-poll-option') : null;
            if (!option) {
                return;
            }
            var quiz = option.closest('.slidelang-quiz');
            if (quiz) {
                revealQuiz(quiz, option);
                return;
            }
            var poll = option.closest('.slidelang-poll');
            if (poll) {
                togglePoll(poll, option);
            }
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
